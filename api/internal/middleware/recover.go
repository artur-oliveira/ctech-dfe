package middleware

import (
	"log/slog"

	"github.com/artur-oliveira/ctech-dfe/api/internal/problem"

	"github.com/gofiber/fiber/v3"
)

// Recover catches panics and returns a 500 Problem response.
func Recover() fiber.Handler {
	return func(c fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic recovered", "path", c.Path(), "panic", r)
				err = c.Status(fiber.StatusInternalServerError).JSON(
					problem.InternalServer("unexpected server error"),
				)
			}
		}()
		return c.Next()
	}
}
