package web

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/oak/crypto-trading-bot/internal/config"
	"github.com/oak/crypto-trading-bot/internal/logger"
	"github.com/oak/crypto-trading-bot/internal/storage"
)

// Server represents the web monitoring server
type Server struct {
	config  *config.Config
	logger  *logger.ColorLogger
	storage *storage.Storage
	hertz   *server.Hertz
}

// NewServer creates a new web monitoring server
func NewServer(cfg *config.Config, log *logger.ColorLogger, db *storage.Storage) *Server {
	h := server.Default(server.WithHostPorts(fmt.Sprintf(":%d", cfg.WebPort)))

	s := &Server{
		config:  cfg,
		logger:  log,
		storage: db,
		hertz:   h,
	}

	s.setupRoutes()

	return s
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// Static pages
	s.hertz.GET("/", s.handleIndex)
	s.hertz.GET("/sessions", s.handleSessions)
	s.hertz.GET("/session/:id", s.handleSessionDetail)
	s.hertz.GET("/stats", s.handleStats)
	s.hertz.GET("/health", s.handleHealth)
}

// handleIndex renders the main dashboard
func (s *Server) handleIndex(ctx context.Context, c *app.RequestContext) {
	stats, err := s.storage.GetSessionStats(s.config.CryptoSymbol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}

	sessions, err := s.storage.GetLatestSessions(10)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}

	tmpl := template.Must(template.ParseFiles("internal/web/templates/index.html"))
	data := map[string]interface{}{
		"Symbol":        s.config.CryptoSymbol,
		"Timeframe":     s.config.CryptoTimeframe,
		"Stats":         stats,
		"Sessions":      sessions,
		"CurrentTime":   time.Now().Format("2006-01-02 15:04:05"),
		"LLMEnabled":    s.config.APIKey != "" && s.config.APIKey != "your_openai_key",
		"TestMode":      s.config.BinanceTestMode,
	}

	// Execute template and render
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		c.JSON(http.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}

	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}

// handleSessions returns JSON list of sessions
func (s *Server) handleSessions(ctx context.Context, c *app.RequestContext) {
	limit := c.DefaultQuery("limit", "20")
	var limitInt int
	fmt.Sscanf(limit, "%d", &limitInt)

	sessions, err := s.storage.GetLatestSessions(limitInt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, utils.H{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

// handleSessionDetail returns details of a specific session
func (s *Server) handleSessionDetail(ctx context.Context, c *app.RequestContext) {
	// This would require implementing GetSessionByID in storage
	c.JSON(http.StatusOK, utils.H{
		"message": "Session detail endpoint - to be implemented",
	})
}

// handleStats returns statistics
func (s *Server) handleStats(ctx context.Context, c *app.RequestContext) {
	stats, err := s.storage.GetSessionStats(s.config.CryptoSymbol)
	if err != nil {
		c.JSON(http.StatusInternalServerError, utils.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// handleHealth returns health status
func (s *Server) handleHealth(ctx context.Context, c *app.RequestContext) {
	c.JSON(http.StatusOK, utils.H{
		"status":  "healthy",
		"time":    time.Now(),
		"version": "1.0.0",
	})
}

// Start starts the web server
func (s *Server) Start() error {
	s.logger.Success(fmt.Sprintf("Web 监控启动: http://localhost:%d", s.config.WebPort))
	s.hertz.Spin()
	return nil
}

// Stop stops the web server
func (s *Server) Stop(ctx context.Context) error {
	return s.hertz.Shutdown(ctx)
}
