
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)


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

	StaticDir string
	Version   string
}


type Log struct {
	Level  string
	Format string 
}


type OneStepGPS struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
	
	MaxAttempts  int
	RetryBackoff time.Duration
	
	SpeedUnit string
	
	DemoMode bool
}


type Database struct {
	Path        string
	AutoMigrate bool
	LogQueries  bool
}


type Poller struct {
	Interval time.Duration
	
	Timeout time.Duration
	
	FailureThreshold int
}


type History struct {
	Enabled       bool
	Retention     time.Duration
	PruneInterval time.Duration
	
	MinDistanceMeters float64
	MaxPointsPerQuery int
}


type Maps struct {
	APIKey string
	MapID  string
}


type Icons struct {
	MaxBytes     int64
	MaxPerUser   int
	AllowedTypes []string
}


type RateLimit struct {
	Enabled bool
	RPS     float64
	Burst   int
}


func Load() (Config, error) {
	
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		if _, statErr := os.Stat(".env"); statErr == nil {
			return Config{}, fmt.Errorf("parse .env: %w", err)
		}
	}

	cfg := Config{
		Env: env("APP_ENV", "development"),
		
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
			Timeout:          envDuration("POLL_TIMEOUT", 0), 
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
			MaxBytes: int64(envInt("ICON_MAX_BYTES", 256*1024)),
			
			MaxPerUser: envInt("ICON_MAX_PER_USER", 50),
			
			AllowedTypes: envList("ICON_ALLOWED_TYPES", []string{"image/png", "image/jpeg", "image/webp", "image/gif"}),
		},
		RateLimit: RateLimit{
			Enabled: envBool("RATE_LIMIT_ENABLED", true),
			RPS:     envFloat("RATE_LIMIT_RPS", 40),
			Burst:   envInt("RATE_LIMIT_BURST", 80),
		},
	}

	
	if cfg.OneStepGPS.APIKey == "" {
		cfg.OneStepGPS.DemoMode = true
	}

	return cfg, cfg.Validate()
}

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
