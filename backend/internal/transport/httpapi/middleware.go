// Package httpapi is the HTTP delivery layer: routing, middleware, request
// binding and response rendering. All business rules live in the service
// package; handlers here stay thin on purpose.
package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// Context keys and headers used across the middleware chain.
const (
	ctxKeyRequestID = "request_id"
	ctxKeyUserID    = "user_id"

	headerRequestID = "X-Request-Id"
	headerUserID    = "X-User-Id"

	// DefaultUserID owns the preferences of anyone who does not identify
	// themselves. Multi-user support is therefore opt-in, not bolted on.
	DefaultUserID = "default"
)

// userIDPattern keeps identifiers safe for logs, SQL parameters and metrics.
var userIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// RequestID assigns every request a correlation id, honouring one supplied by
// an upstream proxy so traces survive across services.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader(headerRequestID))
		if id == "" || len(id) > 64 {
			id = newRequestID()
		}
		c.Set(ctxKeyRequestID, id)
		c.Writer.Header().Set(headerRequestID, id)
		c.Next()
	}
}

// UserContext resolves the caller's preference scope. The browser generates a
// stable id and sends it as a header; anything unparseable falls back to the
// shared default rather than failing the request.
func UserContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		candidate := strings.TrimSpace(c.GetHeader(headerUserID))
		if candidate == "" {
			candidate = strings.TrimSpace(c.Query("userId"))
		}
		if !userIDPattern.MatchString(candidate) {
			candidate = DefaultUserID
		}
		c.Set(ctxKeyUserID, candidate)
		c.Next()
	}
}

// RequestLogger emits one structured line per request.
func RequestLogger(log *slog.Logger, skipPaths ...string) gin.HandlerFunc {
	skip := make(map[string]bool, len(skipPaths))
	for _, p := range skipPaths {
		skip[p] = true
	}

	return func(c *gin.Context) {
		if skip[c.Request.URL.Path] {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"bytes", c.Writer.Size(),
			"ip", c.ClientIP(),
			"request_id", RequestIDOf(c),
			"user", UserIDOf(c),
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "error", c.Errors.String())
		}

		switch {
		case status >= 500:
			log.Error("request failed", attrs...)
		case status >= 400:
			log.Warn("request rejected", attrs...)
		default:
			log.Info("request", attrs...)
		}
	}
}

// Recovery converts a panic into a 500 without taking the process down.
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Error("panic recovered",
					"error", recovered,
					"path", c.Request.URL.Path,
					"request_id", RequestIDOf(c))
				writeError(c, http.StatusInternalServerError, "internal_error",
					"Something went wrong while handling the request.", "")
				c.Abort()
			}
		}()
		c.Next()
	}
}

// CORS answers preflights and echoes only origins we trust. A literal "*" in
// the configuration allows any origin, which is convenient for local demos and
// should not be used in production.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(allowedOrigins))
	wildcard := false
	for _, origin := range allowedOrigins {
		origin = strings.TrimRight(strings.TrimSpace(origin), "/")
		if origin == "*" {
			wildcard = true
			continue
		}
		if origin != "" {
			allowed[strings.ToLower(origin)] = true
		}
	}

	return func(c *gin.Context) {
		origin := strings.TrimRight(c.GetHeader("Origin"), "/")
		if origin != "" && (wildcard || allowed[strings.ToLower(origin)]) {
			h := c.Writer.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Vary", "Origin")
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", strings.Join([]string{
				"Content-Type", "Accept", headerUserID, headerRequestID,
			}, ", "))
			h.Set("Access-Control-Expose-Headers", headerRequestID)
			h.Set("Access-Control-Max-Age", "600")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// SecurityHeaders applies conservative defaults for an API surface.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		c.Next()
	}
}

// visitor is one rate-limited client.
type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// clientKey identifies the caller for rate-limiting purposes.
//
// Behind Cloud Run (and any L7 proxy) the TCP peer is the load balancer, so
// c.ClientIP() returns one address for the entire internet and a single token
// bucket throttles every user at once. The real client is the first entry of
// X-Forwarded-For. That entry is caller-controlled and therefore spoofable,
// which is a deliberate trade: a spoofed key degrades to "no rate limit for
// that attacker", whereas the shared key degrades to "the whole user base is
// throttled". Only the second failure mode takes the product down, and the
// platform's own edge is the real DoS boundary.
func clientKey(c *gin.Context) string {
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		if first, _, found := strings.Cut(xff, ","); found || first != "" {
			if ip := strings.TrimSpace(first); ip != "" {
				return ip
			}
		}
	}
	return c.ClientIP()
}

// RateLimit applies a per-client token bucket to state-changing requests.
// Idle buckets are evicted so the map cannot grow without bound.
//
// Safe, cacheable reads are deliberately exempt: they are served from an
// in-memory snapshot, they are the entire crowd path, and throttling them
// turns a traffic spike into an outage. Writes touch SQLite and are where
// abuse actually costs something.
func RateLimit(rps float64, burst int, skipPaths ...string) gin.HandlerFunc {
	if rps <= 0 {
		rps = 40
	}
	if burst <= 0 {
		burst = int(rps * 2)
	}
	skip := make(map[string]bool, len(skipPaths))
	for _, p := range skipPaths {
		skip[p] = true
	}

	var mu sync.Mutex
	visitors := make(map[string]*visitor)

	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			cutoff := time.Now().Add(-5 * time.Minute)
			mu.Lock()
			for ip, v := range visitors {
				if v.lastSeen.Before(cutoff) {
					delete(visitors, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		if skip[c.Request.URL.Path] {
			c.Next()
			return
		}
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}

		ip := clientKey(c)
		mu.Lock()
		v, ok := visitors[ip]
		if !ok {
			v = &visitor{limiter: rate.NewLimiter(rate.Limit(rps), burst)}
			visitors[ip] = v
		}
		v.lastSeen = time.Now()
		allowed := v.limiter.Allow()
		mu.Unlock()

		if !allowed {
			c.Writer.Header().Set("Retry-After", "1")
			writeError(c, http.StatusTooManyRequests, "rate_limited",
				"Too many requests. Please slow down.", "")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequestIDOf returns the correlation id of the current request.
func RequestIDOf(c *gin.Context) string {
	if v, ok := c.Get(ctxKeyRequestID); ok {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

// UserIDOf returns the preference scope of the current request.
func UserIDOf(c *gin.Context) string {
	if v, ok := c.Get(ctxKeyUserID); ok {
		if id, ok := v.(string); ok && id != "" {
			return id
		}
	}
	return DefaultUserID
}

func newRequestID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(buf)
}
