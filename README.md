# order-matching-engine

## File Structure

```
order-matching-engine/
├── cmd/
│   └── ome/
│       └── main.go
├── internal/
│   ├── api/
│   │   ├── middleware.go
│   │   ├── http/
│   │   │   └── handler.go
│   │   └── ws/
│   │       └── handler.go
│   ├── config/
│   │   └── config.go
│   ├── db/
│   ├── engine/
│   │   ├── matcher.go
│   │   ├── matcher_test.go
│   │   └── orderbook.go
│   └── util/
│       ├── auth.go
│       └── logger.go
├── pkg/
│   └── models/
│       ├── order.go
│       └── trade.go
├── .git/
├── .gitignore
├── go.mod
├── go.sum
└── README.md
```
