package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

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
	c.Status(http.StatusOK)
}

func (s *Server) GetEventsHandler(c *gin.Context) {
	c.Status(http.StatusOK)
}
