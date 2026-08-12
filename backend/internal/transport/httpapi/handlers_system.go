package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"fleetview/internal/transport/dto"
)


func (h *Handlers) RuntimeConfig(c *gin.Context) {
	c.JSON(http.StatusOK, dto.RuntimeConfig{
		GoogleMapsAPIKey: h.cfg.Maps.APIKey,
		GoogleMapsMapID:  h.cfg.Maps.MapID,
		RefreshSeconds:   int(h.cfg.Poller.Interval.Seconds()),
		RealtimeEnabled:  h.hub != nil,
		DemoMode:         h.cfg.OneStepGPS.DemoMode,
		Provider:         h.poller.Health().Provider,
		Version:          h.cfg.Version,
		MaxIconBytes:     h.cfg.Icons.MaxBytes,
	})
}


func (h *Handlers) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": h.cfg.Version,
		"time":    h.clock.Now().UTC(),
	})
}


func (h *Handlers) Readyz(c *gin.Context) {
	health := h.poller.Health()
	snapshot := h.devices.Snapshot()

	body := gin.H{
		"provider":            health.Provider,
		"devices":             len(snapshot.Devices),
		"stale":               snapshot.Stale,
		"lastSuccess":         nullableTime(health.LastSuccess),
		"lastAttempt":         nullableTime(health.LastAttempt),
		"consecutiveFailures": health.ConsecutiveFailures,
		"totalPolls":          health.TotalPolls,
		"totalFailures":       health.TotalFailures,
		"pollIntervalSeconds": health.IntervalSeconds,
		"realtimeClients":     h.wsClients(),
	}
	if health.LastError != "" {
		body["lastError"] = health.LastError
	}

	switch {
	case snapshot.Empty():
		body["status"] = "starting"
		c.JSON(http.StatusServiceUnavailable, body)
	case !health.Healthy:
		body["status"] = "degraded"
		c.JSON(http.StatusServiceUnavailable, body)
	default:
		body["status"] = "ready"
		c.JSON(http.StatusOK, body)
	}
}

func (h *Handlers) wsClients() int {
	if h.hub == nil {
		return 0
	}
	return h.hub.ClientCount()
}

func nullableTime(t interface{ IsZero() bool }) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t
}
