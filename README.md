# order-matching-engine

## File Structure

```
order-matching-engine/
│
├── cmd/
│   └── ome/                     # Executable name (Order Matching Engine)
│       └── main.go              # App entrypoint
│
├── internal/
│   ├── engine/                  # Core OME logic
│   │   ├── orderbook.go         # In-memory order book (sorted by price/time)
│   │   └── matcher.go           # Matching algorithm (price-time priority)
│   │
│   ├── api/                     # Transport layer (REST, WebSocket)
│   │   ├── http/                # REST endpoints
│   │   │   └── handler.go
│   │   ├── ws/                  # WebSocket endpoints
│   │   │   └── handler.go
│   │   └── middleware.go
│   │
│   ├── storage/                 # Data persistence (DB, caching)
│   │   ├── mongo_store.go
│   │   └── scylla_store.go
│   │
│   ├── config/                  # Configuration loading
│   │   └── config.go
│   │
│   ├── util/                    # Generic utilities (logging, auth, etc.)
│   │   ├── logger.go
│   │   └── auth.go
│   │
│   └── test/                    # Tests for core and API
│       ├── matcher_test.go
│       └── api_test.go
│
├── pkg/                         # Public reusable packages
│   └── models/                  # Domain models
│       ├── order.go             # Order struct
│       └── trade.go             # Trade struct
│
├── .gitignore
├── go.mod
├── go.sum
└── README.md
```
