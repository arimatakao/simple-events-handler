package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/arimatakao/simple-events-handler/internal/database"
)

type Server struct {
	port                int
	l                   *slog.Logger
	httpRequestCounter  *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec

	db database.Service
}

func NewServer(logger *slog.Logger) *http.Server {
	port, _ := strconv.Atoi(os.Getenv("PORT"))
	basePath := os.Getenv("BASE_PATH")
	idleTimeout, _ := strconv.Atoi(os.Getenv("IDLE_TIMEOUT_SECONDS"))
	readTimeout, _ := strconv.Atoi(os.Getenv("READ_TIMEOUT_SECONDS"))
	writeTimeout, _ := strconv.Atoi(os.Getenv("WRITE_TIMEOUT_SECONDS"))

	NewServer := &Server{
		port: port,
		l:    logger,

		db: database.New(),
	}

	// Declare Server config
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", NewServer.port),
		Handler:      NewServer.RegisterRoutes(basePath),
		IdleTimeout:  time.Duration(idleTimeout) * time.Second,
		ReadTimeout:  time.Duration(readTimeout) * time.Second,
		WriteTimeout: time.Duration(writeTimeout) * time.Second,
	}

	return server
}
