// Package middleware provides a collection of Fiber middleware components
// for building HTTP servers with standardized behavior.
//
// The middleware components in this package handle common cross-cutting
// concerns such as request logging, error handling, tracing, recovery from
// panics, timeout management, metadata propagation, and error alerting.
// They are designed to work with the server package and follow a consistent
// priority-based execution order.
//
// Middleware Execution Order:
//
// Each middleware declares a Priority value that determines its execution order:
//
//   - Recovery (1000): Catches panics in the middleware chain
//   - Tracing (900): Creates spans for request tracing
//   - Timeout (800): Applies timeouts to request contexts
//   - MetaInject (700): Injects metadata into the request context
//   - Alerting (600): Sends alerts for internal server errors
//   - Logger (500): Logs request and response details
//   - ErrorHandler (400): Converts errors to standardized responses
//
// Higher priority values are executed earlier in the request pipeline.
//
// Usage Example:
//
//	app := fiber.New()
//	server.ApplyMiddlewares(app, []server.Middleware{
//		middleware.NewRecoveryMW(logger),
//		middleware.NewTracingMW(),
//		middleware.NewTimeoutMW(5 * time.Second),
//		middleware.NewMetaInjectMW("my-service", "1.0.0"),
//		middleware.NewAlertingMW(logger, alertProvider),
//		middleware.NewLoggerMW(logger),
//		middleware.NewErrorHandlerMW(false),
//	})
//
// Alternatively, each middleware can be applied individually:
//
//	app.Use(middleware.NewRecoveryMW(logger).Handler)
package middleware
