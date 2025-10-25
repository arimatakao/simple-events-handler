package server

import (
	"fmt"
	"net/http"
	"strconv"
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
	c.Status(http.StatusOK)
}
