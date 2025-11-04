# order-matching-engine

## File Structure
```
order-matching-engine/
│
├── cmd/
│   └── server/              # Main entrypoint
│       └── main.go
│
├── internal/
│   ├── core/                # Core OME logic
│   │   ├── order.go
│   │   ├── trade.go
│   │   ├── orderbook.go
│   │   └── matcher.go
│   │
│   ├── api/                 # REST + WebSocket handlers
│   │   ├── rest/
│   │   │   └── handler.go
│   │   ├── websocket/
│   │   │   └── handler.go
│   │   └── middleware.go
│   │
│   ├── persistence/         # DB layer
│   │   ├── mongo.go
│   │   └── scylla.go
│   │
│   ├── config/              # Configuration
│   │   └── config.go
│   │
│   ├── utils/               # Helpers (logging, auth, etc.)
│   │   ├── logger.go
│   │   └── auth.go
│   │
│   └── tests/               # Unit/integration tests
│       ├── matcher_test.go
│       └── api_test.go
│
├── pkg/                     # Shared packages (optional)
│   └── models/              # Structs used across layers
│       ├── order.go
│       └── trade.go
│
├── go.mod
├── go.sum
└── [README.md]
```