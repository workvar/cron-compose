package api

import (
	"github.com/gofiber/fiber/v3"

	"github.com/croncompose/croncompose/control-plane/internal/hostmetrics"
)

// hostMetricsHandler serves GET /system/host — control-plane machine usage.
func hostMetricsHandler() fiber.Handler {
	return func(c fiber.Ctx) error {
		return c.JSON(hostmetrics.Collect())
	}
}
