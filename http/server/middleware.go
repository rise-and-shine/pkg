package server

import (
	"sort"

	"github.com/gofiber/fiber/v2"
)

// Middleware represents an HTTP middleware with a priority for ordering.
//
// Priority determines the order in which middlewares are applied: higher values are applied first.
// Handler is the Fiber-compatible middleware function.
type Middleware struct {
	Priority int
	Handler  fiber.Handler
}

// ByOrder implements sort.Interface for []Middleware based on the Priority field.
type ByOrder []Middleware

func (b ByOrder) Len() int { return len(b) }

func (b ByOrder) Swap(i, j int) { b[i], b[j] = b[j], b[i] }

func (b ByOrder) Less(i, j int) bool { return b[i].Priority > b[j].Priority }

// applyMiddlewares registers the provided middlewares to the Fiber app in priority order.
//
// Middlewares with higher Priority are applied before those with lower Priority. Nil handlers are skipped.
func applyMiddlewares(app *fiber.App, middlewares []Middleware) {
	sort.Sort(ByOrder(middlewares))
	for _, mw := range middlewares {
		if mw.Handler == nil {
			continue
		}
		app.Use(mw.Handler)
	}
}
