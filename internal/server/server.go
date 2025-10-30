package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
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

	corsAllowOrigins     []string
	corsAllowMethods     []string
	corsAllowHeaders     []string
	corsAllowCredentials bool
}

func splitAndTrim(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if t := strings.TrimSpace(part); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func NewServer(logger *slog.Logger) *http.Server {
	port, _ := strconv.Atoi(os.Getenv("PORT"))
	basePath := os.Getenv("BASE_PATH")
	idleTimeout, _ := strconv.Atoi(os.Getenv("IDLE_TIMEOUT_SECONDS"))
	readTimeout, _ := strconv.Atoi(os.Getenv("READ_TIMEOUT_SECONDS"))
	writeTimeout, _ := strconv.Atoi(os.Getenv("WRITE_TIMEOUT_SECONDS"))

	originsEnv := os.Getenv("CORS_ALLOW_ORIGINS")
	if originsEnv == "" {
		originsEnv = "http://localhost:3000"
	}
	methodsEnv := os.Getenv("CORS_ALLOW_METHODS")
	if methodsEnv == "" {
		methodsEnv = "GET,POST"
	}
	headersEnv := os.Getenv("CORS_ALLOW_HEADERS")
	if headersEnv == "" {
		headersEnv = "Accept,Authorization,Content-Type"
	}
	allowCreds := false
	if v := os.Getenv("CORS_ALLOW_CREDENTIALS"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			allowCreds = b
		}
	}

	NewServer := &Server{
		port: port,
		l:    logger,

		db: database.New(),

		// set parsed CORS values
		corsAllowOrigins:     splitAndTrim(originsEnv),
		corsAllowMethods:     splitAndTrim(methodsEnv),
		corsAllowHeaders:     splitAndTrim(headersEnv),
		corsAllowCredentials: allowCreds,
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
