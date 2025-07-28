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
		ErrorHandler:             customErrorHandler(),
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

// Start begins listening for incoming HTTP requests on the configured address.
func (s *HTTPServer) Start() error {
	return s.router.Listen(s.listenAddr)
}

// Stop gracefully stops the server, allowing for ongoing requests to complete.
func (s *HTTPServer) Stop() error {
	return s.router.Shutdown()
}

// customErrorHandler returns a Fiber error handler that ensures consistent error responses.
//
// If the response status code is already set to an error (>= 400), it does not override it.
// Otherwise, it delegates to Fiber's default error handler.
func customErrorHandler() fiber.ErrorHandler {
	return func(ctx *fiber.Ctx, err error) error {
		r := ctx.Response()
		if r != nil && r.StatusCode() >= 400 {
			return nil
		}

		return fiber.DefaultErrorHandler(ctx, err)
	}
}
