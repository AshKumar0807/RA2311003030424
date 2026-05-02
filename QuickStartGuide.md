# RA2311003030424 — Backend Track (Go / Gin)

## Repository Structure

```
RA2311003030424/
├── logging_middleware/               # Reusable Go logging package(log middle ware)
│   ├── logger.go                     # Core Log() + convenience helpers
│   └── logger_test.go
├── vehicle_maintence_scheduler/    # Vehicle maintence REST API
│   ├── config/      domain/
│   ├── handler/     middleware/
│   ├── repository/  service/
│   └── main.go  (port 8080)
├── notification_app_be/              # Notification delivery service
│   ├── domain/  handler/  service/
│   └── main.go  (port 8081)
├── go.mod
├── .gitignore
├── notification_system_design.md
└── README.md
```

## Quick Start

```bash
git clone https://github.com/AshKumar0807/RA2311003030424 && cd RA2311003030424
go mod tidy

# Vehicle Scheduler
LOG_API_TOKEN=<token> go run ./vehicle_maintence_scheduler/main.go

# Notification Backend
LOG_API_TOKEN=<token> go run ./notification_app_be/main.go

# Tests
go test ./logging_middleware/... -v
```

## Logging Middleware

```go
logger := logging.New(os.Getenv("LOG_API_TOKEN"))
logger.Log("backend", "error", "handler", "received string, expected bool")
logger.Fatal("db", "Critical database connection failure.")
logger.Info("service", "vehicle registered: id=abc registration=MH12AB1234")
```

Target: `POST http://20.207.122.201/evaluation-service/logs`
