package httpserver

import (
	"math"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/baozhenglab/go-sdk/v2/logger"
)

// A Logger Middleware for gin engine,
// it keep our logs in formatted.
// Just demo, need check for better interface
func Logger(log logger.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// other handler can change c.Path so:
		path := c.Path()
		start := time.Now()
		stop := time.Since(start)
		latency := int(math.Ceil(float64(stop.Nanoseconds()) / 1000.0))
		statusCode := c.Response().StatusCode()
		clientIP := c.IP()
		clientUserAgent := string(c.Request().Header.UserAgent())
		referer := string(c.Request().Header.Referer())
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		dataLength := c.Response().Header.ContentLength()
		if dataLength < 0 {
			dataLength = 0
		}

		entry := log.Withs(logger.Fields{
			"hostname":   hostname,
			"statusCode": statusCode,
			"latency":    latency, // time to process
			"clientIP":   clientIP,
			"method":     c.Method(),
			"path":       path,
			"referer":    referer,
			"dataLength": dataLength,
			"userAgent":  clientUserAgent,
		})
		msg := ""
		if statusCode > 499 {
			entry.Error(msg)
		} else if statusCode > 399 {
			entry.Warn(msg)
		} else {
			entry.Info(msg)
		}
		return c.Next()
	}
}
