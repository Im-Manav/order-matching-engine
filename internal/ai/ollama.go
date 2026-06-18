package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Im-Manav/ome/pkg/logger"
	"github.com/Im-Manav/ome/pkg/models"
	"go.uber.org/zap"
)

// Client talks to an OpenAI-compatible LLM endpoint.
// Ollama exposes this API locally — same shape works for
// Groq or any other OpenAI-compatible provider by changing
// BaseURL and APIKey.
type Client struct {
	baseURL    string
	model      string
	apiKey     string
	httpClient *http.Client
}

func NewClient(baseURL, model, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		model:   model,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // local LLM inference can be slow
		},
	}
}

// generateRequest matches Ollama's /api/generate endpoint.
type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	Format string `json:"format,omitempty"` // "json" forces structured output
}

type generateResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// PredictPrice sends recent OHLCV candles and order book context to the
// LLM and asks for a structured short-term direction prediction.
//
// This is pattern-reasoning over real market data, not a magic price oracle —
// the value is in the architecture (data → inference → event → UI),
// not in prediction accuracy.
func (c *Client) PredictPrice(
	ctx context.Context,
	symbol string,
	candles []models.OHLCV,
	bestBid, bestAsk float64,
) (*models.Prediction, error) {
	prompt := buildPrompt(symbol, candles, bestBid, bestAsk)

	reqBody := generateRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
		Format: "json", // Ollama supports forcing valid JSON output
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		c.baseURL+"/api/generate",
		bytes.NewReader(data),
	)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llm returned status %d", resp.StatusCode)
	}

	var genResp generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return nil, fmt.Errorf("decode llm response: %w", err)
	}

	prediction, err := parsePrediction(symbol, genResp.Response)
	if err != nil {
		return nil, fmt.Errorf("parse prediction: %w", err)
	}

	logger.Info("prediction generated",
		zap.String("symbol", symbol),
		zap.String("direction", prediction.Direction),
		zap.Int("confidence", prediction.Confidence),
	)

	return prediction, nil
}

// buildPrompt constructs the LLM prompt from market data.
// Keeping this in one place makes it easy to tune prompt quality
// without touching the request/response plumbing.
func buildPrompt(symbol string, candles []models.OHLCV, bestBid, bestAsk float64) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("You are analyzing %s market data.\n\n", symbol))
	sb.WriteString("Recent 1-minute candles (oldest first):\n")
	for _, c := range candles {
		sb.WriteString(fmt.Sprintf(
			" time=%s open=%.2f high=%.2f low=%.2f close=%.2f volume=%.4f\n",
			c.Time.Format("15:04"), c.Open, c.High, c.Low, c.Close, c.Volume,
		))
	}

	sb.WriteString(fmt.Sprintf("\nCurrent order book: best_bid=%.2f best_ask=%.2f\n\n", bestBid, bestAsk))

	sb.WriteString(`Based on this data, predict the likely price direction over the next 5 candles.
	Consider trend, momentum, and volume.
Respond ONLY with valid JSON in this exact format, no other text:
{"direction": "up", "confidence": 65, "reasoning": "one sentence explanation"}

direction must be exactly one of: "up", "down", "sideways"
confidence is an integer 0-100`)

	return sb.String()
}

// parsePrediction extracts the structured prediction from the LLM's
// raw text response. LLMs sometimes wrap JSON in markdown code fences
// or add stray text — this handles the common cases defensively.
func parsePrediction(symbol, raw string) (*models.Prediction, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var parsed struct {
		Direction  string `json:"direction"`
		Confidence int    `json:"confidence"`
		Reasoning  string `json:"reasoning"`
	}

	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("invalid json from llm: %w (raw: %s)", err, raw)
	}

	// Validate direction — fall back to "sideways" for unexpected values
	// rather than propagating garbage to the dashboard.
	switch parsed.Direction {
	case "up", "down", "sideways":
	default:
		parsed.Direction = "sideways"
	}

	if parsed.Confidence < 0 {
		parsed.Confidence = 0
	}
	if parsed.Confidence > 100 {
		parsed.Confidence = 100
	}

	return &models.Prediction{
		Symbol:      symbol,
		Direction:   parsed.Direction,
		Confidence:  parsed.Confidence,
		Reasoning:   parsed.Reasoning,
		GeneratedAt: time.Now().UTC(),
	}, nil
}
