FROM golang:1.24.5 AS builder

ARG CGO_ENABLED=0
WORKDIR /app

COPY . .
RUN go mod tidy
RUN go build -o ./simple-events-handler ./cmd/api/main.go

# 2 stage. Runner
FROM scratch
COPY --from=builder /app/simple-events-handler /bin/simple-events-handler

EXPOSE 8080

ENTRYPOINT ["/bin/simple-events-handler"]