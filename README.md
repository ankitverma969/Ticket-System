# Go Ticket System

This repository contains the Go backend for the Ticket System.

## Environment Variables
Copy `.env.example` to `.env` and set the configuration values:
- `PORT`: HTTP server port (defaults to `8080`)
- `DATABASE_URL`: PostgreSQL connection string (for upcoming stages)
- `JWT_SECRET`: Secret key for signing JWT tokens (for upcoming stages)
- `JWT_EXPIRATION`: Token validity duration (e.g. `24h`)

## Development
Run the server:
```bash
go run ./cmd/server
```

Run tests:
```bash
go test ./...
```
