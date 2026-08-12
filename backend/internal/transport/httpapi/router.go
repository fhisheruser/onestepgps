package httpapi

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"

	"fleetview/internal/config"
)

// RouterDeps are the constructor arguments of NewRouter.
type RouterDeps struct {
	Handlers *Handlers
	Config   config.Config
	Logger   *slog.Logger
}

// NewRouter builds the fully wired HTTP engine.
//
// Route shapes avoid Gin's static-vs-parameter conflicts by construction:
// exports live under /export and bulk preference actions under their own verbs
// instead of colliding with /devices/:deviceId.
func NewRouter(deps RouterDeps) *gin.Engine {
	cfg := deps.Config
	log := deps.Logger
	h := deps.Handlers

	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()

	// Gin trusts all proxies by default, which makes ClientIP() spoofable.
	// Trust only what the operator configured.
	if err := engine.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		log.Warn("failed to set trusted proxies", "error", err)
	}
	engine.RedirectTrailingSlash = true
	engine.HandleMethodNotAllowed = true

	engine.Use(
		Recovery(log),
		RequestID(),
		UserContext(),
		SecurityHeaders(),
		CORS(cfg.AllowedOrigins),
		RequestLogger(log, "/healthz", "/readyz"),
		// Cloud Run does not compress for us. The JS bundle is ~77 KB raw and
		// ~24 KB gzipped, and that saving is per visitor. The WebSocket route
		// is excluded because compressing a hijacked connection breaks the
		// upgrade.
		gzip.Gzip(gzip.DefaultCompression, gzip.WithExcludedPaths([]string{"/api/v1/ws"})),
		StaticCacheHeaders(),
	)
	if cfg.RateLimit.Enabled {
		engine.Use(RateLimit(cfg.RateLimit.RPS, cfg.RateLimit.Burst, "/healthz", "/readyz"))
	}

	engine.GET("/healthz", h.Healthz)
	engine.GET("/readyz", h.Readyz)

	api := engine.Group("/api/v1")
	{
		api.GET("/config", h.RuntimeConfig)

		api.GET("/devices", h.ListDevices)
		api.GET("/devices/:deviceId", h.GetDevice)
		api.GET("/devices/:deviceId/history", h.GetDeviceHistory)

		api.GET("/fleet/summary", h.GetSummary)
		api.GET("/export/devices.csv", h.ExportCSV)

		api.GET("/preferences", h.GetPreferences)
		api.PUT("/preferences/settings", h.UpdateSettings)
		api.POST("/preferences/reset", h.ResetPreferences)
		api.POST("/preferences/reorder", h.ReorderDevices)
		api.PUT("/preferences/devices/:deviceId", h.UpsertDevicePreference)
		api.DELETE("/preferences/devices/:deviceId", h.DeleteDevicePreference)
		api.POST("/preferences/devices/:deviceId/icon", h.UploadIcon)
		api.DELETE("/preferences/devices/:deviceId/icon", h.DeleteIcon)

		api.GET("/icons/:iconId", h.GetIcon)

		api.GET("/ws", h.ServeWebSocket)
	}

	registerStatic(engine, cfg.StaticDir, log)
	registerFallback(engine, cfg.StaticDir)
	return engine
}

// StaticCacheHeaders makes repeat visits nearly free.
//
// Vite fingerprints every file under /assets, so those bytes can never change
// under the same URL and are safe to cache forever. index.html and the service
// worker must never be cached, or a deploy would be invisible to anyone with a
// warm cache.
func StaticCacheHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		switch {
		case strings.HasPrefix(path, "/assets/"):
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		case path == "/" || path == "/index.html" || path == "/sw.js":
			c.Header("Cache-Control", "no-cache")
		}
		c.Next()
	}
}

// registerStatic optionally serves a built single-page app from the same
// origin, which turns the whole product into one deployable binary. In the
// Docker Compose setup Nginx does this instead and StaticDir stays empty.
func registerStatic(engine *gin.Engine, dir string, log *slog.Logger) {
	if dir == "" {
		return
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		log.Warn("static directory not served", "dir", dir, "error", err)
		return
	}

	engine.Static("/assets", filepath.Join(dir, "assets"))
	for _, name := range []string{"favicon.svg", "favicon.ico", "manifest.webmanifest", "sw.js", "robots.txt"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			engine.StaticFile("/"+name, path)
		}
	}
	log.Info("serving static frontend", "dir", dir)
}

// registerFallback returns index.html for unknown non-API paths so client-side
// routing works on a hard refresh, and a JSON 404 for everything else.
func registerFallback(engine *gin.Engine, dir string) {
	indexPath := ""
	if dir != "" {
		candidate := filepath.Join(dir, "index.html")
		if _, err := os.Stat(candidate); err == nil {
			indexPath = candidate
		}
	}

	engine.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		isAPI := strings.HasPrefix(path, "/api/") ||
			path == "/healthz" || path == "/readyz"

		if indexPath != "" && !isAPI && c.Request.Method == http.MethodGet {
			c.File(indexPath)
			return
		}
		writeError(c, http.StatusNotFound, "not_found", "No route matches this request.", "")
	})

	engine.NoMethod(func(c *gin.Context) {
		writeError(c, http.StatusMethodNotAllowed, "method_not_allowed",
			"That method is not supported on this route.", "")
	})
}
