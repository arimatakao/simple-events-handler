# Simple Event Handler

Simple Event Handler is a small, opinionated Go application that accepts events over an HTTP API, performs light processing, and runs periodic aggregation tasks in the background.

Key characteristics:
- HTTP API server that receives and handles event payloads.
- Background aggregator (cron-like) that runs periodic jobs (start/stop controlled in code).
- Structured JSON logging used throughout the server and background tasks.
- Graceful shutdown support to let in-flight requests finish and to stop background jobs cleanly.


## Examples usage

Below are simple examples showing how to send events to the API and how to query them. These examples assume the server is running locally on port 8080 and the BASE_PATH is /api.

1) Create an event (POST /api/events)

Request:
```sh
curl -i -X POST "http://localhost:8080/api/events" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": 123,
    "action": "login",
    "metadata": {"page":"/home"}
  }'
```

Successful response (status only, empty body):
```
HTTP/1.1 201 Created
Content-Type: application/json
...headers...
```

Notes:
- The server returns 201 Created with an empty body on success (the handler sets StatusCreated).
- If the JSON is invalid or required fields are missing you'll get a 400 response with details.

Example error (invalid JSON):
```
HTTP/1.1 400 Bad Request
Content-Type: application/json

{"error":"invalid request","details":"invalid character '...' looking for beginning of object key string"}
```

Example error (validation failed):
```
HTTP/1.1 400 Bad Request
Content-Type: application/json

{"error":"validation failed","details":"user_id must be a positive integer"}
```

2) Query events (GET /api/events)

Basic query (time range required):
```sh
curl -i "http://localhost:8080/api/events?user_id=123&from=2025-01-01T00:00:00Z&to=2025-01-02T00:00:00Z"
```

Successful response (200 OK, JSON array of events):
```
HTTP/1.1 200 OK
Content-Type: application/json

[
  {
    "id": 1,
    "user_id": 123,
    "action": "login",
    "metadata": {"page":"/home"},
    "created_at": "2025-01-01T12:34:56Z"
  },
  {
    "id": 2,
    "user_id": 123,
    "action": "purchase",
    "metadata": {"item":"/home"},
    "created_at": "2025-01-01T13:00:00Z"
  }
]
```

Notes:
- The from and to parameters accept multiple common time formats (RFC3339, "2006-01-02 15:04:05", date-only etc.).
- The server also attempts to unescape URL-encoded timestamps (useful if your client double-encodes query params).

Example error (missing/invalid times):
```
HTTP/1.1 400 Bad Request
Content-Type: application/json

{"error":"invalid time format","details":"invalid from parameter: unrecognized time format: \"...\""}
```

Example error (server/database issue):
```
HTTP/1.1 500 Internal Server Error
Content-Type: application/json

{"error":"failed to fetch events"}
```


## Getting Started Developing

These instructions will get you a copy of the project up and running on your local machine for development and testing purposes. See deployment for notes on how to deploy the project on a live system.

## Environment variables

The application uses environment variables to configure the HTTP server, aggregation scheduler, time zone, and database connection. For development you can copy .env.example to .env and adjust values.

- PORT (int, default: 8080)
  - TCP port the HTTP server will listen on.

- BASE_PATH (string, default: /api)
  - Base route prefix for all HTTP endpoints (e.g. /api). If empty, routes are served from root.

- AGGREGATION_INTERVAL_SECONDS (int, default: 30)
  - How often (in seconds) the background aggregator should run. Must be a positive integer. The aggregator will run approximately every N seconds.

- IDLE_TIMEOUT_SECONDS (int, default: 60)
  - HTTP server idle timeout in seconds (max time to keep idle connections open).

- READ_TIMEOUT_SECONDS (int, default: 10)
  - Maximum duration in seconds for reading the entire request, including the body.

- WRITE_TIMEOUT_SECONDS (int, default: 30)
  - Maximum duration in seconds before timing out writes of the response.

- TZ (string, example: Europe/Kiev)
  - Time zone used by containers / scripts that respect TZ. Not strictly required by the app, but useful in Docker setups and examples.

Database connection variables (used by the app and docker-compose):

- DB_HOST (string, default: localhost)
  - Hostname or IP of the Postgres server.

- DB_PORT (int, default: 5432)
  - Port on which Postgres is listening. In docker-compose this maps host port to container 5432.

- DB_DATABASE (string, example: events)
  - Name of the Postgres database to connect to.

- DB_USERNAME (string)
  - Database username.

- DB_PASSWORD (string)
  - Database password.

- DB_SCHEMA (string, default: public)
  - Postgres search_path/schema to use (the code appends this to the connection string).

Notes and behavior:
- The application reads values with os.Getenv and falls back to simple defaults where appropriate. Numeric values are parsed with strconv.Atoi; invalid numeric values will typically fall back to the default or log a warning (see source).
- For local development you can populate a .env file from .env.example. When running in Docker, docker-compose reads environment variables or uses the values from an .env file in the compose directory.
- Keep credentials (DB_USERNAME, DB_PASSWORD) out of version control; use environment-specific secrets or a vault in production.

## MakeFile

Run build make command with tests
```sh
make all
```

Build the application
```sh
make build
```

Run the application
```sh
make run
```
Create DB container
```sh
make docker-run
```

Shutdown DB Container
```sh
make docker-down
```

DB Integrations Test:
```sh
make itest
```

Run the test suite:
```sh
make test
```

Clean up binary from the last build:
```sh
make clean
```
## Development notes

Main components (high level):
- cmd/api — program entrypoint that sets up logging, starts the HTTP server and the aggregator, and handles graceful shutdown.
- internal/server — HTTP server and routes/handlers that accept events.
- internal/aggregator — periodic job scheduler/worker that performs aggregation or background processing.
- Tests and integration tests exercised via Makefile targets.

- The server uses structured JSON logging to stdout for easy consumption by log collectors or local debugging.
- The aggregator exposes Start and Stop so the application can control its lifecycle (see cmd/api for graceful shutdown handling).
- The application is intentionally small and modular to make it easy to extend (add handlers, persistence, metrics, etc.).