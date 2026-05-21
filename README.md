# Online Parking — Microservices Platform

A production-grade online parking management system built with a microservices architecture. The platform allows users to browse parking lots, reserve spots, process payments, and receive email notifications — all coordinated across independent services communicating via gRPC and NATS JetStream.

---

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Services](#services)
- [Tech Stack](#tech-stack)
- [API Reference](#api-reference)
- [Project Structure](#project-structure)
- [Getting Started](#getting-started)
- [Environment Variables](#environment-variables)
- [Monitoring](#monitoring)
- [Testing](#testing)
- [Authors](#authors)

---

## Architecture Overview

```
                          ┌─────────────────────────────────────┐
                          │         Client (Browser / App)       │
                          └──────────────────┬──────────────────┘
                                             │ HTTP :8080
                          ┌──────────────────▼──────────────────┐
                          │           API Gateway (Nginx)        │
                          │         Routes /api/* → services     │
                          └──┬──────┬──────┬──────┬──────┬──────┘
                             │      │      │      │      │
              ┌──────────────┘      │      │      │      └──────────────┐
              │                     │      │      │                     │
    ┌─────────▼────────┐  ┌─────────▼──┐  │  ┌───▼──────────┐  ┌──────▼──────────┐
    │   Auth Service   │  │User Service│  │  │Booking Service│  │Payment Service  │
    │     :8082        │  │   :8081    │  │  │    :8084      │  │    :8086        │
    │  gRPC :9092      │  │ gRPC :9091 │  │  │  gRPC :9094  │  │  gRPC :9096     │
    └──────────────────┘  └────────────┘  │  └───────────────┘  └─────────────────┘
                                          │
                             ┌────────────▼──────────────┐
                             │  Parking Service  :8085    │
                             │       gRPC :9095           │
                             └───────────────────────────┘

    ┌─────────────────────────────────────────────────────────────┐
    │                    Notification Service :8083               │
    │    NATS subscriber → sends emails via SMTP (Gmail/Google)  │
    └─────────────────────────────────────────────────────────────┘

    ┌──────────────────┐   ┌──────────────────┐   ┌──────────────────┐
    │  PostgreSQL :5432 │   │   Redis :6379    │   │  NATS  :4222     │
    │  (6 databases)   │   │  (6 DBs cache)   │   │  JetStream MQ    │
    └──────────────────┘   └──────────────────┘   └──────────────────┘

    ┌──────────────────┐   ┌──────────────────┐
    │ Prometheus :9090 │   │  Grafana :3000   │
    │  metrics scrape  │   │   dashboards     │
    └──────────────────┘   └──────────────────┘
```

### Communication patterns

| Pattern | Used for |
|---------|----------|
| **HTTP REST** | External client ↔ API Gateway ↔ Services |
| **gRPC** | Internal service-to-service calls (Booking → User, Booking → Parking, etc.) |
| **NATS JetStream** | Async events: user registered, booking confirmed, payment completed → Notification Service |

---

## Services

### Auth Service (`/api/auth/`)
Handles authentication and session management. Issues JWT access + refresh tokens, supports email verification, password reset via SMTP, and session revocation.

- JWT access tokens (15 min TTL), refresh tokens (30 days)
- Email verification flow with tokenized links
- Forgot/reset password via SMTP
- Redis-backed session store
- Publishes events to NATS on registration/login

### User Service (`/api/users/`)
Manages user profiles, admin operations (ban, verify), and exposes a gRPC interface consumed by Booking and Payment services.

- CRUD for user profiles
- Batch user fetch via gRPC (used by Booking to enrich data)
- Email verification endpoint
- Prometheus metrics on `:9201`

### Parking Service (`/api/parking/`)
Core domain service managing parking lots, individual spots, and tariff calculations.

- Create/get parking lots
- Manage spots (create, reserve, release, update status, delete)
- Tariff management (per-parking, with price calculation endpoint)
- Fully exposed via gRPC for Booking and Payment services

### Booking Service (`/api/bookings/`)
Orchestrates the booking lifecycle. Calls Parking Service (gRPC) to validate and reserve spots, calls User Service (gRPC) to validate users. Publishes booking events to NATS.

**Booking states:** `pending → confirmed → active → completed` / `cancelled`

- Quick booking (auto-assign spot) and standard booking
- Start / confirm / cancel / complete flows
- Redis cache for active booking lookups
- Publishes events on status changes

### Payment Service (`/api/payments/`)
Processes payments for bookings. Integrates with Booking and Parking services via HTTP to compute the final price, creates payment records, and publishes payment events to NATS.

- Create payment by booking ID (auto-fetches booking details + tariff)
- Cancel payment
- Payment history by user or booking

### Notification Service (`/api/notifications/`)
Listens to NATS events from all other services and sends transactional emails via SMTP (Gmail / Google Workspace). Stores notification history in PostgreSQL.

- Sends emails: welcome, email verification, booking confirmed/cancelled, payment receipt, password reset
- Bulk email support
- Notification history + unread count per user
- User preferences (subscribe/unsubscribe)
- SMS and push stubs for future extension

### API Gateway (Nginx)
Nginx reverse-proxy that routes all `/api/*` traffic to the appropriate upstream service and serves the React frontend SPA for all other paths.

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.22+ |
| HTTP framework | Gin, `net/http` |
| gRPC | `google.golang.org/grpc` |
| Message queue | NATS 2.10 with JetStream |
| Database | PostgreSQL 16 |
| Cache | Redis 7 |
| Migrations | SQL migration files (per service) |
| API Gateway | Nginx 1.27 |
| Frontend | React + Vite (JS) |
| Containerization | Docker + Docker Compose |
| Observability | Prometheus + Grafana + Node Exporter |
| Email | SMTP (Gmail / Google Workspace) |

---

## API Reference

All endpoints are accessible through the API Gateway at `http://localhost:8080`.

### Auth — `/api/auth/`

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/auth/register` | Register a new user |
| `POST` | `/api/auth/login` | Login, returns access + refresh token |
| `POST` | `/api/auth/refresh` | Refresh access token |
| `POST` | `/api/auth/logout` | Invalidate current session |
| `GET` | `/api/auth/session` | Get current session info |
| `POST` | `/api/auth/revoke-all-sessions` | Revoke all active sessions |
| `GET` | `/api/auth/verify-email` | Verify email from link |
| `POST` | `/api/auth/forgot-password` | Request password reset email |
| `POST` | `/api/auth/reset-password` | Reset password with token |
| `POST` | `/api/auth/change-password` | Change password (authenticated) |

### Users — `/api/users/`

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/users/` | List all users |
| `POST` | `/api/users/` | Create user |
| `GET` | `/api/users/:id` | Get user by ID |
| `PUT` | `/api/users/:id` | Update user |
| `DELETE` | `/api/users/:id` | Delete user |
| `POST` | `/api/users/:id/verify` | Mark email as verified |
| `POST` | `/api/users/:id/ban` | Ban user |
| `GET` | `/api/users/:id/stats` | Get user statistics |
| `POST` | `/api/users/batch` | Batch fetch users by IDs |

### Parking — `/api/parking/`

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/parking/parkings` | Create a parking lot |
| `GET` | `/api/parking/parkings` | List all parking lots |
| `GET` | `/api/parking/parkings/:id` | Get parking lot details |
| `POST` | `/api/parking/spots` | Add a spot to a parking lot |
| `GET` | `/api/parking/spots/:id` | Get spot details |
| `GET` | `/api/parking/parkings/:id/spots` | List spots for a parking lot |
| `PATCH` | `/api/parking/spots/:id/status` | Update spot status |
| `POST` | `/api/parking/spots/:id/reserve` | Reserve a spot |
| `POST` | `/api/parking/spots/:id/release` | Release a reserved spot |
| `DELETE` | `/api/parking/spots/:id` | Delete a spot |
| `POST` | `/api/parking/tariffs` | Create tariff for parking lot |
| `GET` | `/api/parking/tariffs/:parking_id` | Get tariff |
| `PATCH` | `/api/parking/tariffs/:parking_id` | Update tariff |
| `GET` | `/api/parking/tariffs/:parking_id/calculate` | Calculate price for duration |

### Bookings — `/api/bookings/`

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/bookings/` | Create booking (specify spot) |
| `POST` | `/api/bookings/quick` | Quick booking (auto-assign spot) |
| `GET` | `/api/bookings/` | List bookings |
| `GET` | `/api/bookings/:id` | Get booking by ID |
| `POST` | `/api/bookings/:id/confirm` | Confirm booking |
| `POST` | `/api/bookings/:id/cancel` | Cancel booking |
| `POST` | `/api/bookings/:id/start` | Start (activate) booking |
| `POST` | `/api/bookings/:id/complete` | Complete booking |

### Payments — `/api/payments/`

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/payments/` | Create payment manually |
| `POST` | `/api/payments/booking/:booking_id` | Create payment for a booking |
| `GET` | `/api/payments/` | List payments |
| `GET` | `/api/payments/:id` | Get payment by ID |
| `GET` | `/api/payments/booking/:booking_id` | Get payment for a booking |
| `POST` | `/api/payments/:id/cancel` | Cancel payment |

### Notifications — `/api/notifications/`

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/api/notifications/email` | Send single email |
| `POST` | `/api/notifications/bulk-email` | Send bulk email |
| `GET` | `/api/notifications/history` | Notification history |
| `GET` | `/api/notifications/unread-count` | Unread notifications count |
| `GET/POST` | `/api/notifications/preferences` | Get/update notification preferences |
| `GET` | `/api/notifications/:id` | Get notification by ID |
| `DELETE` | `/api/notifications/:id` | Delete notification |

---

## Project Structure

```
AP2_Final_OnlineParking/
├── docker-compose.yml              # Full stack orchestration
├── .env                            # Root-level shared env (SMTP, JWT, DB URLs)
├── postgres/
│   └── init.sql                    # Initializes all 6 databases on first run
├── frontend/
│   ├── src/App.jsx                 # React SPA
│   └── dist/                       # Built assets (served by Nginx)
├── monitoring/
│   ├── prometheus/
│   │   ├── prometheus.yml          # Scrape configs for all services
│   │   └── rules/                  # Recording rules
│   └── grafana/
│       └── provisioning/           # Auto-provisioned dashboards & datasources
├── services/
│   ├── api-gateway/
│   │   └── nginx.conf/nginx.conf   # Reverse-proxy routing
│   ├── auth-service/               # JWT auth, sessions, password reset
│   ├── user-service/               # User CRUD + gRPC server
│   ├── parking-service/            # Parking lots, spots, tariffs + gRPC server
│   ├── booking-service/            # Booking lifecycle, gRPC client for user+parking
│   ├── payment-service/            # Payment processing + NATS publisher
│   └── notification-service/       # NATS subscriber, SMTP email sender
└── infra/
    └── terraform/                  # (Terraform config placeholder)
```

Each service follows Go clean architecture:

```
services/<name>/
├── cmd/app/main.go         # Entrypoint
├── internal/
│   ├── app/app.go          # Wire-up: DB, Redis, NATS, HTTP, gRPC servers
│   ├── config/config.go    # Env-based config struct
│   ├── domain/             # Domain models and business rules
│   ├── repository/         # PostgreSQL data access layer
│   ├── service/            # Business logic + unit tests
│   ├── transport/
│   │   ├── http/           # REST handlers (Gin or net/http)
│   │   └── grpc/           # gRPC server implementation
│   └── publisher/          # NATS event publishers
├── migrations/             # SQL migration files
├── Dockerfile
└── .env                    # Service-specific env (overridden by docker-compose)
```

---

## Getting Started

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) 24+
- [Docker Compose](https://docs.docker.com/compose/) v2+
- Git

### 1. Clone the repository

```bash
git clone https://github.com/your-org/AP2_Final_OnlineParking.git
cd AP2_Final_OnlineParking
```

### 2. Configure environment variables

Copy the example files and fill them in (see the [Environment Variables](#environment-variables) section below):

```bash
# Root .env (shared config — JWT secret, SMTP credentials, DB URLs)
cp .env.example .env

# Per-service .env files (ports, Redis DB, service URLs)
cp services/auth-service/.env.example        services/auth-service/.env
cp services/user-service/.env.example        services/user-service/.env
cp services/notification-service/.env.example services/notification-service/.env
cp services/parking-service/.env.example     services/parking-service/.env
cp services/booking-service/.env.example     services/booking-service/.env
cp services/payment-service/.env.example     services/payment-service/.env
```

> The most critical values to set are **`JWT_SECRET`** and the **SMTP credentials** in the root `.env`.

### 3. Build and start all services

```bash
docker compose up --build
```

Docker Compose will:
1. Start PostgreSQL and run `postgres/init.sql` to create all 6 databases
2. Start Redis (6 logical DBs) and NATS with JetStream enabled
3. Build and start all 6 Go microservices
4. Start Nginx as the API Gateway, serving the frontend SPA
5. Start Prometheus and Grafana for observability

### 4. Verify the stack is up

```bash
# API Gateway (entry point for everything)
curl http://localhost:8080/api/auth/health

# Individual service health checks
curl http://localhost:8082/health   # auth-service
curl http://localhost:8081/health   # user-service
curl http://localhost:8083/health   # notification-service
curl http://localhost:8084/health   # booking-service
curl http://localhost:8085/health   # parking-service
curl http://localhost:8086/health   # payment-service
```

### 5. Access the services

| Service | URL |
|---------|-----|
| Frontend + API Gateway | http://localhost:8080 |
| Grafana dashboards | http://localhost:3000 (admin / admin) |
| Prometheus | http://localhost:9090 |
| NATS monitoring | http://localhost:8222 |

### Quick smoke test

```bash
# 1. Register a user
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"StrongPass1!","name":"John Doe"}'

# 2. Login
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"StrongPass1!"}'

# 3. Create a parking lot
curl -X POST http://localhost:8080/api/parking/parkings \
  -H "Content-Type: application/json" \
  -d '{"name":"Central Parking","address":"123 Main St","total_spots":50}'

# 4. Quick-book a spot
curl -X POST http://localhost:8080/api/bookings/quick \
  -H "Content-Type: application/json" \
  -d '{"user_id":"<user_id>","parking_id":"<parking_id>","start_time":"2026-06-01T10:00:00Z","end_time":"2026-06-01T12:00:00Z"}'
```

---

## Environment Variables

### Root `.env` — shared across services

```dotenv
# ── PostgreSQL ────────────────────────────────────────────────────────────────
# Connection details used by services that read from .env directly
USER_DB_HOST=postgres
USER_DB_PORT=5432
USER_DB_USER=parking
USER_DB_PASSWORD=your_db_password_here
USER_DB_NAME=user_service
USER_DB_SSLMODE=disable

AUTH_DATABASE_URL=postgres://parking:your_db_password_here@postgres:5432/auth_service?sslmode=disable
NOTIFICATION_DATABASE_URL=postgres://parking:your_db_password_here@postgres:5432/notification_service?sslmode=disable

# ── JWT ───────────────────────────────────────────────────────────────────────
# Generate with: openssl rand -base64 32
JWT_SECRET=replace_with_a_strong_random_base64_string

# ── SMTP (Email) ──────────────────────────────────────────────────────────────
# For Gmail: enable 2FA, then create an App Password at myaccount.google.com/apppasswords
# For Google Workspace: same approach with your work email
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your_email@gmail.com
SMTP_PASSWORD=your_gmail_app_password
SMTP_FROM=your_email@gmail.com

# ── Frontend ──────────────────────────────────────────────────────────────────
FRONTEND_URL=http://localhost:3000
```

### `services/auth-service/.env`

```dotenv
APP_PORT=8082
GRPC_PORT=9092

DATABASE_URL=postgres://parking:your_db_password_here@postgres:5432/auth_service?sslmode=disable

REDIS_HOST=redis
REDIS_PORT=6379
REDIS_DB=1
REDIS_PASSWORD=

# Same value as root JWT_SECRET
JWT_SECRET=replace_with_a_strong_random_base64_string

# Token lifetimes
ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=720h
VERIFICATION_TOKEN_TTL=24h
PASSWORD_RESET_TTL=1h
```

### `services/user-service/.env`

```dotenv
APP_PORT=8081
GRPC_PORT=9091
METRICS_PORT=9201

DB_HOST=postgres
DB_PORT=5432
DB_USER=parking
DB_PASSWORD=your_db_password_here
DB_NAME=user_service
DB_SSLMODE=disable

REDIS_HOST=redis
REDIS_PORT=6379
REDIS_DB=0
REDIS_PASSWORD=
```

### `services/notification-service/.env`

```dotenv
HTTP_PORT=8083
GRPC_PORT=9093
METRICS_PORT=9203

DATABASE_URL=postgres://parking:your_db_password_here@postgres:5432/notification_service?sslmode=disable

REDIS_HOST=redis
REDIS_PORT=6379
REDIS_DB=2
REDIS_PASSWORD=

NATS_URL=nats://nats:4222

# SMTP — same as root .env
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your_email@gmail.com
SMTP_PASSWORD=your_gmail_app_password
SMTP_FROM=your_email@gmail.com

FRONTEND_URL=http://localhost:3000
```

### `services/parking-service/.env`

```dotenv
HTTP_PORT=8085
GRPC_PORT=9095

DB_URL=postgres://parking:your_db_password_here@postgres:5432/parking_service?sslmode=disable

REDIS_HOST=redis
REDIS_PORT=6379
REDIS_DB=4
REDIS_PASSWORD=
```

### `services/booking-service/.env`

```dotenv
APP_PORT=8084
GRPC_PORT=9094

DB_HOST=postgres
DB_PORT=5432
DB_USER=parking
DB_PASSWORD=your_db_password_here
DB_NAME=booking_service
DB_SSLMODE=disable

REDIS_HOST=redis
REDIS_PORT=6379
REDIS_DB=3
REDIS_PASSWORD=

NATS_URL=nats://nats:4222

# Internal gRPC addresses (service names from docker-compose)
USER_GRPC_ADDR=user-service:9091
PARKING_GRPC_ADDR=parking-service:9095
PARKING_HTTP_URL=http://parking-service:8085
```

### `services/payment-service/.env`

```dotenv
HTTP_PORT=8086
GRPC_PORT=9096

DB_HOST=postgres
DB_PORT=5432
DB_USER=parking
DB_PASSWORD=your_db_password_here
DB_NAME=payment_service
DB_SSLMODE=disable

REDIS_HOST=redis
REDIS_PORT=6379
REDIS_DB=5
REDIS_PASSWORD=

NATS_URL=nats://nats:4222

# Internal HTTP addresses for cross-service calls
BOOKING_SERVICE_URL=http://booking-service:8084
PARKING_SERVICE_URL=http://parking-service:8085
USER_SERVICE_URL=http://user-service:8081
```

---

## Monitoring

The stack includes a fully pre-configured observability setup.

### Prometheus
Scrapes metrics from all services that expose a `/metrics` endpoint (User Service, Notification Service, and others via Prometheus client). Accessible at **http://localhost:9090**.

### Grafana
Pre-provisioned with the **Online Parking Overview** dashboard. Accessible at **http://localhost:3000** with credentials `admin / admin`.

The dashboard includes:
- Request rates and latencies per service
- Active bookings and payment counts
- NATS message throughput
- Node-level system metrics (CPU, memory, disk) via Node Exporter

### Node Exporter
Exposes host-level system metrics at **http://localhost:9100**, scraped by Prometheus.

---

## Testing

Each service contains unit tests and, where applicable, integration tests.

```bash
# Run tests for a specific service
cd services/auth-service
go test ./...

cd services/booking-service
go test ./...

cd services/notification-service
go test ./...

cd services/parking-service
go test ./...

cd services/payment-service
go test ./...

cd services/user-service
go test ./...

# Run all tests from root (requires Go installed locally)
find services -name "*.go" -path "*/service/*_test.go" \
  | xargs -I{} dirname {} \
  | sort -u \
  | xargs -I{} sh -c 'cd {} && go test ./... -v'
```

Test coverage includes:
- **Unit tests** for all service-layer business logic (mocked repositories)
- **Integration tests** for HTTP handlers in Parking and Payment services
- **Domain tests** for booking and payment domain models

---

## Authors

Built with ❤️ by:

**Artyom Safaryan · Alikhan Faizrakhman · Daniyar Ayazbaev**