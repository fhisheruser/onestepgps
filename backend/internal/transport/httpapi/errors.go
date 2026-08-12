package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"fleetview/internal/domain"
	"fleetview/internal/transport/dto"
)

// writeError renders the single error shape the frontend understands.
func writeError(c *gin.Context, status int, code, message, field string) {
	c.JSON(status, dto.Envelope{Error: &dto.APIError{
		Code:    code,
		Message: message,
		Field:   field,
	}})
}

// respondError maps a domain or infrastructure error onto an HTTP response.
// Internal details are logged, never returned: clients get a stable code and a
// message safe to show a user.
func respondError(c *gin.Context, log *slog.Logger, err error) {
	switch {
	case err == nil:
		return

	case errors.Is(err, context.Canceled):
		// The client hung up; nothing to report.
		c.Abort()
		return

	case errors.Is(err, domain.ErrNotFound):
		writeError(c, http.StatusNotFound, "not_found", "The requested resource does not exist.", "")

	case errors.Is(err, domain.ErrUpstreamAuth):
		log.Error("upstream rejected our credentials", "error", err, "request_id", RequestIDOf(c))
		writeError(c, http.StatusBadGateway, "upstream_auth_failed",
			"The GPS provider rejected the configured API key.", "")

	case errors.Is(err, domain.ErrUnavailable):
		writeError(c, http.StatusServiceUnavailable, "data_unavailable",
			"Live device data is not available yet. Please retry shortly.", "")

	default:
		if verr, ok := domain.AsValidationError(err); ok {
			writeError(c, http.StatusUnprocessableEntity, "validation_failed", verr.Message, verr.Field)
			return
		}
		log.Error("unhandled request error",
			"error", err,
			"path", c.Request.URL.Path,
			"request_id", RequestIDOf(c))
		writeError(c, http.StatusInternalServerError, "internal_error",
			"Something went wrong while handling the request.", "")
	}
}
