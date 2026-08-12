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


type RouterDeps struct {
	Handlers *Handlers
	Config   config.Config
	Logger   *slog.Logger
}


func NewRouter(deps RouterDeps) *gin.Engine {
	cfg := deps.Config
	log := deps.Logger
	h := deps.Handlers

	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()

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
