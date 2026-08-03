package middleware

import (
	"time"

	sharedctx "facturacion-service/internal/shared/context"
	"facturacion-service/pkg/ulid"
	"github.com/gofiber/fiber/v3"
)

// RequestContext injects request_id (ULID), client IP, user agent, and timestamp
// into the Fiber context locals for downstream use.
func RequestContext() fiber.Handler {
	return func(c fiber.Ctx) error {
		requestID := ulid.New().String()
		now := time.Now().UTC()

		clientType := sharedctx.ExtractClientType(c)

		reqCtx := &sharedctx.RequestContext{
			RequestID:  requestID,
			IP:         sharedctx.ExtractIP(c),
			UserAgent:  sharedctx.ExtractUserAgent(c),
			Timestamp:  now,
			ClientType: clientType,
		}

		c.Locals(sharedctx.KeyRequestContext, reqCtx)
		c.Locals(sharedctx.KeyRequestID, requestID)
		c.Locals(sharedctx.KeyTimestamp, now.Format(time.RFC3339))
		c.Locals(sharedctx.KeyClientType, clientType)

		c.Set("X-Request-ID", requestID)

		return c.Next()
	}
}
