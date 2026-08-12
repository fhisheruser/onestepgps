// Package config loads and validates runtime configuration from the
// environment (optionally seeded from a .env file). Nothing else in the
// application reads os.Getenv, so every knob is discoverable in one place.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config is the fully resolved application configuration.
type Config struct {
	Env             string
	HTTPAddr        string
	ShutdownTimeout time.Duration
	AllowedOrigins  []string
	TrustedProxies  []string
	Log             Log
	OneStepGPS      OneStepGPS
	Database        Database
	Poller          Poller
	History         History
	Maps            Maps
	Icons           Icons
	RateLimit       RateLimit
	// StaticDir optionally serves a built frontend from the same origin,
	// collapsing the whole product into a single binary. Empty in the Compose
	// setup, where Nginx serves the SPA.
	StaticDir string
	Version   string
}

// Log controls the structured logger.
type Log struct {
	Level  string
	Format string // "json" or "text"
}

// OneStepGPS holds upstream provider settings.
type OneStepGPS struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
	// MaxAttempts includes the first try, so 3 means "try, retry, retry".
	MaxAttempts int
	RetryBackoff time.Duration
	// SpeedUnit labels the speed values returned by the provider. It is a
	// label only: no conversion happens on the server, the UI converts to the
	// user's preferred unit.
	SpeedUnit string
	// DemoMode swaps the live client for a deterministic simulator.
	DemoMode bool
}

// Database holds persistence settings.
type Database struct {
	Path        string
	AutoMigrate bool
	LogQueries  bool
}

// Poller controls the background refresh loop.
type Poller struct {
	Interval time.Duration
	// Timeout bounds a single refresh cycle including the client's retries.
	// Zero means "three times the interval".
	Timeout time.Duration
	// FailureThreshold is how many consecutive failures may occur before the
	// service reports itself unhealthy.
	FailureThreshold int
}

// History controls breadcrumb persistence.
type History struct {
	Enabled       bool
	Retention     time.Duration
	PruneInterval time.Duration
	// MinDistanceMeters suppresses breadcrumbs for a parked vehicle.
	MinDistanceMeters float64
	MaxPointsPerQuery int
}

// Maps holds the values handed to the browser at runtime. The Google Maps
// JavaScript key is public by design (it is restricted by HTTP referrer);
// the OneStepGPS key by contrast never leaves the server.
type Maps struct {
	APIKey string
	MapID  string
}

// Icons constrains marker image uploads.
type Icons struct {
	MaxBytes     int64
	AllowedTypes []string
}

// RateLimit protects the API from runaway clients.
type RateLimit struct {
	Enabled bool
	RPS     float64
	Burst   int
}

// Load resolves configuration from .env (if present) plus the environment.
// Later sources win: real environment variables always override the file.
func Load() (Config, error) {
	// A missing .env is normal in Docker/production; only surface real errors.
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		if _, statErr := os.Stat(".env"); statErr == nil {
			return Config{}, fmt.Errorf("parse .env: %w", err)
		}
	}

	cfg := Config{
		Env:             env("APP_ENV", "development"),
		// Cloud Run, Heroku and friends inject PORT and expect the process to
		// listen on it. HTTP_ADDR stays the explicit override for everything else.
		HTTPAddr:        env("HTTP_ADDR", ":"+env("PORT", "8080")),
		ShutdownTimeout: envDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
		AllowedOrigins:  envList("CORS_ALLOWED_ORIGINS", []string{"http://localhost:5173", "http://localhost:3000", "http://localhost:8081"}),
		TrustedProxies:  envList("TRUSTED_PROXIES", nil),
		StaticDir:       env("STATIC_DIR", ""),
		Version:         env("APP_VERSION", "1.0.0"),
		Log: Log{
			Level:  env("LOG_LEVEL", "info"),
			Format: env("LOG_FORMAT", "text"),
		},
		OneStepGPS: OneStepGPS{
			BaseURL:      env("ONESTEPGPS_BASE_URL", "https://track.onestepgps.com/v3/api/public/device"),
			APIKey:       env("ONESTEPGPS_API_KEY", ""),
			Timeout:      envDuration("ONESTEPGPS_TIMEOUT", 12*time.Second),
			MaxAttempts:  envInt("ONESTEPGPS_MAX_ATTEMPTS", 3),
			RetryBackoff: envDuration("ONESTEPGPS_RETRY_BACKOFF", 500*time.Millisecond),
			SpeedUnit:    env("ONESTEPGPS_SPEED_UNIT", "km/h"),
			DemoMode:     envBool("DEMO_MODE", false),
		},
		Database: Database{
			Path:        env("DB_PATH", "data/fleetview.db"),
			AutoMigrate: envBool("DB_AUTO_MIGRATE", true),
			LogQueries:  envBool("DB_LOG_QUERIES", false),
		},
		Poller: Poller{
			Interval:         envDuration("POLL_INTERVAL", 10*time.Second),
			Timeout:          envDuration("POLL_TIMEOUT", 0), // 0 => 3x interval
			FailureThreshold: envInt("POLL_FAILURE_THRESHOLD", 3),
		},
		History: History{
			Enabled:           envBool("HISTORY_ENABLED", true),
			Retention:         envDuration("HISTORY_RETENTION", 24*time.Hour),
			PruneInterval:     envDuration("HISTORY_PRUNE_INTERVAL", 30*time.Minute),
			MinDistanceMeters: envFloat("HISTORY_MIN_DISTANCE_METERS", 15),
			MaxPointsPerQuery: envInt("HISTORY_MAX_POINTS", 500),
		},
		Maps: Maps{
			APIKey: env("GOOGLE_MAPS_API_KEY", ""),
			MapID:  env("GOOGLE_MAPS_MAP_ID", "DEMO_MAP_ID"),
		},
		Icons: Icons{
			MaxBytes:     int64(envInt("ICON_MAX_BYTES", 256*1024)),
			// SVG is deliberately absent: an attacker-supplied SVG served from
			// our own origin can execute script. Add it here only behind a
			// sanitiser you trust.
			AllowedTypes: envList("ICON_ALLOWED_TYPES", []string{"image/png", "image/jpeg", "image/webp", "image/gif"}),
		},
		RateLimit: RateLimit{
			Enabled: envBool("RATE_LIMIT_ENABLED", true),
			RPS:     envFloat("RATE_LIMIT_RPS", 40),
			Burst:   envInt("RATE_LIMIT_BURST", 80),
		},
	}

	// Running without credentials is a supported mode: fall back to the
	// simulator rather than refusing to boot, so the dashboard is always
	// explorable. The caller logs a prominent warning.
	if cfg.OneStepGPS.APIKey == "" {
		cfg.OneStepGPS.DemoMode = true
	}

	return cfg, cfg.Validate()
}

// Validate rejects configurations that could not possibly work.
func (c Config) Validate() error {
	if c.HTTPAddr == "" {
		return fmt.Errorf("HTTP_ADDR must not be empty")
	}
	if c.Poller.Interval < time.Second {
		return fmt.Errorf("POLL_INTERVAL must be at least 1s, got %s", c.Poller.Interval)
	}
	if c.OneStepGPS.MaxAttempts < 1 {
		return fmt.Errorf("ONESTEPGPS_MAX_ATTEMPTS must be at least 1")
	}
	if !c.OneStepGPS.DemoMode && c.OneStepGPS.BaseURL == "" {
		return fmt.Errorf("ONESTEPGPS_BASE_URL must not be empty")
	}
	if c.Database.Path == "" {
		return fmt.Errorf("DB_PATH must not be empty")
	}
	return nil
}

// IsProduction reports whether the app runs with production defaults.
func (c Config) IsProduction() bool {
	return strings.EqualFold(c.Env, "production") || strings.EqualFold(c.Env, "prod")
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, err := strconv.Atoi(env(key, "")); err == nil {
		return v
	}
	return fallback
}

func envFloat(key string, fallback float64) float64 {
	if v, err := strconv.ParseFloat(env(key, ""), 64); err == nil {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v, err := strconv.ParseBool(env(key, "")); err == nil {
		return v
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := env(key, "")
	if raw == "" {
		return fallback
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return d
	}
	// Bare numbers are interpreted as seconds, which is what most people mean
	// when they write POLL_INTERVAL=10.
	if secs, err := strconv.Atoi(raw); err == nil {
		return time.Duration(secs) * time.Second
	}
	return fallback
}

func envList(key string, fallback []string) []string {
	raw := env(key, "")
	if raw == "" {
		return fallback
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}
