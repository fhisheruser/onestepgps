
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


const (
	ctxKeyRequestID = "request_id"
	ctxKeyUserID    = "user_id"

	headerRequestID = "X-Request-Id"
	headerUserID    = "X-User-Id"


	DefaultUserID = "default"
)


var userIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

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


const appCSP = "default-src 'self'; " +
	"base-uri 'self'; " +
	"object-src 'none'; " +
	"frame-ancestors 'none'; " +
	"form-action 'self'; " +
	"script-src 'self' 'unsafe-inline' https://maps.googleapis.com https://maps.gstatic.com; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob: https://*.googleapis.com https://*.gstatic.com https://*.google.com https://*.ggpht.com; " +
	"font-src 'self' data:; " +
	"connect-src 'self' https://maps.googleapis.com https://*.googleapis.com wss: ; " +
	"worker-src 'self' blob:"

func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", appCSP)
		
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=(), usb=(), interest-cohort=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin-allow-popups")
		c.Next()
	}
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}


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


func RequestIDOf(c *gin.Context) string {
	if v, ok := c.Get(ctxKeyRequestID); ok {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}


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
