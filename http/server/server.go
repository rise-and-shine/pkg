// Package server provides a configurable HTTP server implementation based on the Fiber framework.
package server

import "github.com/gofiber/fiber/v2"

// HTTPServer provides an HTTP server with configurable middleware.
//
// The server is built on top of the Fiber framework and supports prioritized middleware registration.
// Use NewHTTPServer to create a new instance.
type HTTPServer struct {
	cfg        Config
	router     *fiber.App
	listenAddr string
}

// NewHTTPServer creates a new HTTPServer with the provided configuration and middleware.
//
// The middlewares slice is applied in order of descending priority. The server uses a custom error handler
// for consistent error responses. Returns a pointer to the initialized HTTPServer.
func NewHTTPServer(cfg Config, middlewares []Middleware) *HTTPServer {
	router := fiber.New(fiber.Config{
		ReadTimeout:              cfg.ReadTimeout,
		WriteTimeout:             cfg.WriteTimeout,
		IdleTimeout:              cfg.IdleTimeout,
		ErrorHandler:             customErrorHandler(cfg.Debug),
		DisableStartupMessage:    true,
		Immutable:                true,
		BodyLimit:                cfg.BodyLimit,
		EnableSplittingOnParsers: true,
	})

	applyMiddlewares(router, middlewares)

	srv := &HTTPServer{
		cfg:        cfg,
		router:     router,
		listenAddr: cfg.Address(),
	}

	return srv
}

// RegisterRouter registers a router with the server using the provided register function.
func (s *HTTPServer) RegisterRouter(registerFunc func(r fiber.Router)) {
	registerFunc(s.router)
}

// GetApp returns the underlying Fiber application instance.
func (s *HTTPServer) GetApp() *fiber.App {
	return s.router
}

// Start begins listening for incoming HTTP requests on the configured address.
func (s *HTTPServer) Start() error {
	return s.router.Listen(s.listenAddr)
}

// Stop gracefully stops the server, allowing for ongoing requests to complete.
func (s *HTTPServer) Stop() error {
	return s.router.Shutdown()
}
