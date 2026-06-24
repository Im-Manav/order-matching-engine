package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"time"
)

// ─── Config ───────────────────────────────────────────────────────────────────

const (
	baseURL       = "http://localhost:8080"
	simEmail      = "simulator@ome.local"
	simPassword   = "simulator_password_123"
	orderInterval = 500 * time.Millisecond // place an order every 500ms
	printInterval = 10                     // print stats every N orders
)

// Symbol defines a tradeable instrument with a realistic seed price
// and volatility. Volatility controls how much the price drifts
// per tick — higher = more dramatic price moves.
type Symbol struct {
	Name       string
	SeedPrice  float64
	Volatility float64 // % per tick, e.g. 0.002 = 0.2%
	MinQty     float64
	MaxQty     float64
}

var symbols = []Symbol{
	{Name: "BTC-USD", SeedPrice: 65000.0, Volatility: 0.003, MinQty: 0.001, MaxQty: 0.5},
	{Name: "ETH-USD", SeedPrice: 3200.0, Volatility: 0.003, MinQty: 0.01, MaxQty: 2.0},
	{Name: "AAPL", SeedPrice: 189.0, Volatility: 0.001, MinQty: 1.0, MaxQty: 20.0},
	{Name: "TSLA", SeedPrice: 245.0, Volatility: 0.002, MinQty: 1.0, MaxQty: 15.0},
}

// ─── Types ────────────────────────────────────────────────────────────────────

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type placeOrderRequest struct {
	Symbol   string  `json:"symbol"`
	Side     int     `json:"side"` // 0=buy 1=sell
	Type     int     `json:"type"` // 0=limit 1=market
	Price    float64 `json:"price"`
	Quantity float64 `json:"quantity"`
}

// ─── Market state ─────────────────────────────────────────────────────────────

// MarketState tracks the simulated mid-price for each symbol
// and drifts it randomly each tick to simulate real price movement.
type MarketState struct {
	prices map[string]float64
	rng    *rand.Rand
}

func NewMarketState() *MarketState {
	prices := make(map[string]float64)
	for _, s := range symbols {
		prices[s.Name] = s.SeedPrice
	}
	return &MarketState{
		prices: prices,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Tick drifts the mid-price using geometric Brownian motion —
// the same model used in the Black-Scholes options pricing formula.
// Price can go up or down by up to `volatility` percent per tick.
// This produces realistic-looking price charts with trends and reversals.
func (m *MarketState) Tick(sym Symbol) float64 {
	current := m.prices[sym.Name]

	// Random walk: +1 or -1, weighted by a small drift bias
	drift := (m.rng.Float64() - 0.49) * 2 // slight upward bias
	change := current * sym.Volatility * drift
	next := current + change

	// Floor at 1% below seed — prevents price from going to zero
	floor := sym.SeedPrice * 0.01
	if next < floor {
		next = floor
	}

	m.prices[sym.Name] = next
	return next
}

// SpreadAround returns bid and ask prices around a mid price.
// Spread is 0.1% of mid — tighter than retail, wider than HFT.
func (m *MarketState) SpreadAround(mid float64) (bid, ask float64) {
	halfSpread := mid * 0.001
	return roundPrice(mid - halfSpread), roundPrice(mid + halfSpread)
}

// RandomQty returns a random quantity between min and max for a symbol.
func (m *MarketState) RandomQty(sym Symbol) float64 {
	raw := sym.MinQty + m.rng.Float64()*(sym.MaxQty-sym.MinQty)
	return roundQty(raw)
}

// ─── HTTP client ──────────────────────────────────────────────────────────────

type SimClient struct {
	token      string
	httpClient *http.Client
}

func NewSimClient() *SimClient {
	return &SimClient{
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *SimClient) post(path string, body any) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	return c.httpClient.Do(req)
}

// Register creates the simulator user account.
// If the account already exists (409), that's fine — just login.
func (c *SimClient) Register() error {
	resp, err := c.post("/auth/register", registerRequest{
		Email:    simEmail,
		Password: simPassword,
	})
	if err != nil {
		return fmt.Errorf("register request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 && resp.StatusCode != 409 {
		return fmt.Errorf("register returned %d", resp.StatusCode)
	}
	return nil
}

// Login authenticates and stores the JWT for subsequent requests.
func (c *SimClient) Login() error {
	resp, err := c.post("/auth/login", loginRequest{
		Email:    simEmail,
		Password: simPassword,
	})
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("login returned %d", resp.StatusCode)
	}

	var lr loginResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return fmt.Errorf("decode login response: %w", err)
	}

	c.token = lr.Token
	return nil
}

// PlaceOrder submits a single order to the gateway.
func (c *SimClient) PlaceOrder(req placeOrderRequest) error {
	resp, err := c.post("/api/v1/orders", req)
	if err != nil {
		return fmt.Errorf("place order request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		return fmt.Errorf("place order returned %d", resp.StatusCode)
	}
	return nil
}

// ─── Simulation logic ─────────────────────────────────────────────────────────

// orderStrategy decides what kind of order to place this tick.
// 70% limit orders (provide liquidity), 30% market orders (take liquidity).
// This ratio produces steady matches — a book of only limit orders
// never trades; a book of only market orders has nothing to match against.
func orderStrategy(rng *rand.Rand) (side int, orderType int, isMarket bool) {
	side = rng.Intn(2) // 0=buy, 1=sell equally likely

	if rng.Float64() < 0.30 {
		return side, 1, true // market order
	}
	return side, 0, false // limit order
}

// ─── Stats ────────────────────────────────────────────────────────────────────

type Stats struct {
	sent   int
	failed int
	start  time.Time
}

func (s *Stats) Print(market *MarketState) {
	elapsed := time.Since(s.start).Seconds()
	rate := float64(s.sent) / elapsed
	fmt.Printf("\n─── Simulator stats ───────────────────────────────\n")
	fmt.Printf("  Orders sent: %d (%.1f/sec)  Failures: %d\n", s.sent, rate, s.failed)
	fmt.Printf("  Current prices:\n")
	for _, sym := range symbols {
		fmt.Printf("    %-10s $%.4f\n", sym.Name, market.prices[sym.Name])
	}
	fmt.Println("───────────────────────────────────────────────────")
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║     OME Market Simulator                 ║")
	fmt.Println("║     Ctrl+C to stop                       ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Printf("\nTarget: %s\n", baseURL)
	fmt.Printf("Symbols: ")
	for _, s := range symbols {
		fmt.Printf("%s($%.0f)", s.Name, s.SeedPrice)
	}
	fmt.Println("\n")

	client := NewSimClient()
	market := NewMarketState()
	stats := Stats{start: time.Now()}

	// ── Auth ─────────────────────────────────────────────────────────────────
	fmt.Print("Registering simulator user... ")
	if err := client.Register(); err != nil {
		fmt.Printf("WARN: %v\n", err)
	} else {
		fmt.Println("OK")
	}

	fmt.Print("Logging in... ")
	for retries := 0; retries < 5; retries++ {
		if err := client.Login(); err != nil {
			fmt.Printf("attempt %d failed: %v, retrying...\n", retries+1, err)
			time.Sleep(2 * time.Second)
			continue
		}
		fmt.Println("OK")
		break
	}
	if client.token == "" {
		fmt.Println("ERROR: could not authenticate. Is the gateway running?")
		os.Exit(1)
	}

	// ── Main simulation loop ──────────────────────────────────────────────────
	ticker := time.NewTicker(orderInterval)
	defer ticker.Stop()

	fmt.Println("Simulation running...\n")

	for range ticker.C {
		// Pick a random symbol for this tick
		sym := symbols[market.rng.Intn(len(symbols))]

		// Advance the price
		mid := market.Tick(sym)
		bid, ask := market.SpreadAround(mid)
		qty := market.RandomQty(sym)

		// Decide order strategy
		side, orderType, isMarket := orderStrategy(market.rng)

		var price float64
		if !isMarket {
			// Limit order: place slightly inside the spread to encourage matching
			if side == 0 { // buy
				price = ask // willing to pay the ask → aggressive buy, likely matches
			} else { // sell
				price = bid // willing to sell at bid → aggressive sell, likely matches
			}
			// 40% of the time place a passive order further from mid
			if market.rng.Float64() < 0.4 {
				offset := mid * 0.005 * market.rng.Float64() // 0-0.5% from mid
				if side == 0 {
					price = roundPrice(bid - offset) // passive buy below bid
				} else {
					price = roundPrice(ask + offset) // passive sell above ask
				}
			}
		}

		req := placeOrderRequest{
			Symbol:   sym.Name,
			Side:     side,
			Type:     orderType,
			Price:    price,
			Quantity: qty,
		}

		if err := client.PlaceOrder(req); err != nil {
			stats.failed++
			fmt.Printf("  [FAIL] %s side=%d type=%d err=%v\n",
				sym.Name, side, orderType, err)
		} else {
			stats.sent++
			typeStr := "LIMIT"
			if isMarket {
				typeStr = "MARKET"
			}
			sideStr := "BUY "
			if side == 1 {
				sideStr = "SELL"
			}
			fmt.Printf("  [%s] %s %s qty=%.4f price=%.2f mid=%.2f\n",
				typeStr, sideStr, sym.Name, qty, price, mid)
		}

		// Print stats periodically
		if stats.sent > 0 && stats.sent%printInterval == 0 {
			stats.Print(market)
		}
	}
}

// Helpers

func roundPrice(p float64) float64 {
	return math.Round(p*100) / 100
}
func roundQty(q float64) float64 {
	return math.Round(q*10000) / 10000
}
