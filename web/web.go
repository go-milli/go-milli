package web

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/aevumio/go-milli/logger"
	"github.com/aevumio/go-milli/publisher"
)

// Service wraps gin.Engine with optional Publisher and standard middleware.
type Service struct {
	engine *gin.Engine
	opts   Options
	pub    publisher.Publisher
}

// NewService creates a new web service.
func NewService(opts ...Option) *Service {
	options := newOptions(opts...)

	if options.Mode != "" {
		gin.SetMode(options.Mode)
	}

	engine := gin.New()
	engine.Use(gin.Recovery())

	s := &Service{
		engine: engine,
		opts:   options,
	}

	// If broker is provided, create publisher
	if options.Broker != nil {
		pubOpts := []publisher.Option{
			publisher.Broker(options.Broker),
		}
		if len(options.PublisherWrappers) > 0 {
			pubOpts = append(pubOpts, publisher.WrapPublisher(options.PublisherWrappers...))
		}
		s.pub = publisher.NewDefaultPublisher(pubOpts...)
	}

	return s
}

// Engine returns the underlying *gin.Engine for route registration.
func (s *Service) Engine() *gin.Engine {
	return s.engine
}

// Publisher returns the optional publisher. nil if no Broker was configured.
func (s *Service) Publisher() publisher.Publisher {
	return s.pub
}

// Run starts the web service and blocks until shutdown signal.
func (s *Service) Run() error {
	// Connect broker if present
	if s.opts.Broker != nil {
		if err := s.opts.Broker.Connect(); err != nil {
			return fmt.Errorf("failed to connect broker: %w", err)
		}
	}

	srv := &http.Server{
		Addr:    s.opts.Address,
		Handler: s.engine,
	}

	logger.Infof("Starting [web] %s on %s", s.opts.Name, s.opts.Address)

	// Start HTTP server in background
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-quit:
		logger.Infof("Received signal %s, shutting down...", sig)
	}

	// Graceful shutdown HTTP
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Errorf("Server forced to shutdown: %v", err)
	}

	// Disconnect broker if present
	if s.opts.Broker != nil {
		if err := s.opts.Broker.Disconnect(); err != nil {
			logger.Errorf("Broker disconnect error: %v", err)
		}
	}

	logger.Infof("Stopped [web] %s", s.opts.Name)
	return nil
}
