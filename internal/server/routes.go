package server

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type AddEventRequest struct {
	UserID   int64             `json:"user_id" binding:"required"`
	Action   string            `json:"action" binding:"required"`
	Metadata map[string]string `json:"metadata"`
}

func (a AddEventRequest) Validate() error {
	if a.UserID <= 0 {
		return fmt.Errorf("user_id must be a positive integer")
	}
	if a.Action == "" {
		return fmt.Errorf("action is required")
	}
	return nil
}

type GetEventsRequest struct {
	UserID *int64
	From   string
	To     string
}

// parseTimeFlexible tries to unescape the input (handles values that were URL-encoded
// multiple times like "%2520") and parse using several common time layouts.
func (r GetEventsRequest) parseTimeFlexible(v string) (*time.Time, error) {
	if v == "" {
		return nil, fmt.Errorf("empty time string")
	}

	// Unescape up to a few times to handle double-encoding like %2520 -> %20 -> space
	uv := v
	for i := 0; i < 3; i++ {
		u, err := url.QueryUnescape(uv)
		if err != nil {
			break
		}
		if u == uv {
			break
		}
		uv = u
	}
	uv = strings.TrimSpace(uv)
	if uv == "" {
		return nil, fmt.Errorf("empty time after unescape")
	}

	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}

	for _, l := range layouts {
		if t, err := time.Parse(l, uv); err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("unrecognized time format: %q", v)
}

func (r *GetEventsRequest) Validate() (*time.Time, *time.Time, error) {
	// user id (if present) must be positive
	if r.UserID != nil && *r.UserID <= 0 {
		return nil, nil, fmt.Errorf("user_id must be a positive integer")
	}
	if r.From == "" {
		return nil, nil, fmt.Errorf("from paramater")
	}

	start, err := r.parseTimeFlexible(r.From)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid from parameter: %w", err)
	}

	end, err := r.parseTimeFlexible(r.To)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid to parameter: %w", err)
	}

	// from must not be after to
	if start.After(*end) {
		return nil, nil, fmt.Errorf("from must be before or equal to to")
	}

	return start, end, nil
}

func (s *Server) RegisterRoutes(basePath string) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"}, // Frontend URL
		AllowMethods:     []string{"GET", "POST"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false, // Enable cookies/auth
	}))

	base := r.Group(basePath)
	base.Use(s.LogMiddleware())
	base.POST("/events", s.AddEventHandler)
	base.GET("/events", s.GetEventsHandler)

	return r
}

func (s *Server) LogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		method := c.Request.Method

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		s.l.Info("HTTP request",
			"method", method,
			"path", path,
			"status", status,
			"duration_sec", duration,
			"client_ip", c.ClientIP(),
		)
	}
}

func (s *Server) AddEventHandler(c *gin.Context) {
	var req AddEventRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "details": err.Error()})
		return
	}

	if err := req.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "validation failed", "details": err.Error()})
		return
	}

	// Insert into DB
	ctx := c.Request.Context()
	id, err := s.db.InsertEvent(ctx, req.UserID, req.Action, req.Metadata)
	if err != nil {
		s.l.Error("failed to insert event", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to insert event"})
		return
	}

	s.l.Info("new event added", "event_id", id)
	c.Status(http.StatusCreated)
}

func (s *Server) GetEventsHandler(c *gin.Context) {
	// Build request from query params
	var req GetEventsRequest

	// optional user_id
	if v := c.Query("user_id"); v != "" {
		uid, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
			return
		}
		req.UserID = &uid
	}

	req.From = c.Query("from")
	req.To = c.Query("to")

	startPtr, endPtr, err := req.Validate()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid time format", "details": err.Error()})
		return
	}

	// Query DB
	ctx := c.Request.Context()
	events, err := s.db.GetEvents(ctx, req.UserID, startPtr, endPtr)
	if err != nil {
		s.l.Error("failed to query events", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch events"})
		return
	}

	// Return JSON array of events
	c.JSON(http.StatusOK, events)
}
