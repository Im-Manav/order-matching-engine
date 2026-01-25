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

In order to start the server, run go run cmd/ome/main.go
Access Swagger UI at http://localhost:8080/swagger/index.html
Make sure the server is also running at the same port as Swagger(In this case 8080)