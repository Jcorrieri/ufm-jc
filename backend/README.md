# Backend README

## Setup

Copy `.example.env` to `.env` and provide every required value. From this directory, run the
database lifecycle commands explicitly before starting the HTTP server:

```
go run ./cmd/migrate
go run ./cmd/seed # optional local sample data
go run ./cmd/server
```

The server command opens the configured SQLite database but never changes its schema or inserts
sample data. Run the migration command after pulling schema changes.

## Project Structure
Technology Stack:
- RestAPI: Gin framework
- Database: SQLite with GORM
- Auth: JWTs with HttpOnly cookies.

Product Structure:

```
.
├── README.md
├── app
│   └── router.go
├── cmd
│   ├── migrate
│   ├── seed
│   └── server
├── config
│   └── config.go
├── database
│   ├── database.go
│   └── seed.go
├── go.mod
├── go.sum
├── handlers
│   ├── auth_handler.go
│   ├── settings_handler.go
│   └── user_handler.go
├── middleware
│   ├── middleware.go
│   └── middleware_test.go
├── models
│   ├── user.go
│   └── user_test.go
├── services
│   ├── auth_service.go
│   └── user_service.go
└── utils
    └── jwt.go
```
