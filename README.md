# Go Ticket System

This repository contains the Go backend for the Ticket System assignment.

## Environment Variables
Copy `.env.example` to `.env` and set configuration values if needed:
- `PORT`: HTTP server port (defaults to `8080`)
- `MONGODB_URI`: MongoDB connection URI (for upcoming database stage)
- `MONGODB_DATABASE`: MongoDB database name (defaults to `ticket_system`)
- `JWT_SECRET`: Secret key for signing JWT tokens (for upcoming authentication stage)
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

Verify health check:
```bash
curl http://localhost:8080/health
```
Response:
```json
{
  "status": "ok"
}
```
