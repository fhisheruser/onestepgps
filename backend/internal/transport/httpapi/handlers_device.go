package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"fleetview/internal/config"
	"fleetview/internal/domain"
	"fleetview/internal/service"
	"fleetview/internal/transport/dto"
	"fleetview/internal/transport/ws"
)


const maxSearchLength = 120


type Handlers struct {
	devices *service.DeviceService
	prefs   *service.PreferenceService
	poller  *service.Poller
	hub     *ws.Hub
	cfg     config.Config
	log     *slog.Logger
	clock   domain.Clock
}


type HandlerDeps struct {
	Devices     *service.DeviceService
	Preferences *service.PreferenceService
	Poller      *service.Poller
	Hub         *ws.Hub
	Config      config.Config
	Logger      *slog.Logger
	Clock       domain.Clock
}


func NewHandlers(deps HandlerDeps) *Handlers {
	if deps.Clock == nil {
		deps.Clock = domain.SystemClock{}
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &Handlers{
		devices: deps.Devices,
		prefs:   deps.Preferences,
		poller:  deps.Poller,
		hub:     deps.Hub,
		cfg:     deps.Config,
		log:     deps.Logger,
		clock:   deps.Clock,
	}
}


func (h *Handlers) ListDevices(c *gin.Context) {
	feed, err := h.devices.Feed(c.Request.Context(), UserIDOf(c), parseDeviceQuery(c))
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, dto.FromFeed(feed, h.clock.Now()))
}


func (h *Handlers) GetDevice(c *gin.Context) {
	device, err := h.devices.Device(c.Request.Context(), UserIDOf(c), c.Param("deviceId"))
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, dto.FromDeviceView(device))
}


func (h *Handlers) GetDeviceHistory(c *gin.Context) {
	minutes := clampInt(queryInt(c, "minutes", 60), 1, 24*60)
	limit := clampInt(queryInt(c, "limit", h.cfg.History.MaxPointsPerQuery), 1, h.cfg.History.MaxPointsPerQuery)

	points, err := h.devices.History(c.Request.Context(), c.Param("deviceId"), time.Duration(minutes)*time.Minute, limit)
	if err != nil {
		respondError(c, h.log, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"deviceId":     c.Param("deviceId"),
		"windowMinutes": minutes,
		"points":       dto.FromHistory(points),
	})
}


func (h *Handlers) GetSummary(c *gin.Context) {
	feed, err := h.devices.Feed(c.Request.Context(), UserIDOf(c), parseDeviceQuery(c))
	if err != nil {
		respondError(c, h.log, err)
		return
	}
	c.JSON(http.StatusOK, dto.FromSummary(feed.Summary))
}


func (h *Handlers) ExportCSV(c *gin.Context) {
	payload, err := h.devices.ExportCSV(c.Request.Context(), UserIDOf(c), parseDeviceQuery(c))
	if err != nil {
		respondError(c, h.log, err)
		return
	}

	filename := fmt.Sprintf("fleetview-devices-%s.csv", h.clock.Now().UTC().Format("20060102-1504"))
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", payload)
}


func (h *Handlers) ServeWebSocket(c *gin.Context) {
	if h.hub == nil {
		writeError(c, http.StatusNotImplemented, "realtime_disabled", "Realtime updates are not enabled.", "")
		return
	}
	h.hub.Serve(c.Writer, c.Request, UserIDOf(c), parseDeviceQuery(c))
}


func parseDeviceQuery(c *gin.Context) domain.DeviceQuery {
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > maxSearchLength {
		search = search[:maxSearchLength]
	}

	q := domain.DeviceQuery{
		Search:        search,
		IncludeHidden: queryBool(c, "includeHidden"),
		OnlyPinned:    queryBool(c, "pinned"),
	}

	switch status := domain.StatusFilter(strings.ToLower(strings.TrimSpace(c.Query("status")))); status {
	case domain.StatusAll, domain.StatusDriving, domain.StatusIdle,
		domain.StatusOff, domain.StatusOnline, domain.StatusOffline:
		q.Status = status
	}

	switch key := domain.SortKey(strings.ToLower(strings.TrimSpace(c.Query("sort")))); key {
	case domain.SortKeyName, domain.SortKeyStatus, domain.SortKeySpeed,
		domain.SortKeyUpdated, domain.SortKeyCustom:
		q.SortKey = key
	}

	switch dir := domain.SortDirection(strings.ToLower(strings.TrimSpace(c.Query("dir")))); dir {
	case domain.SortAsc, domain.SortDesc:
		q.SortDirection = dir
	}

	return q
}

func queryBool(c *gin.Context, key string) bool {
	value, err := strconv.ParseBool(strings.TrimSpace(c.Query(key)))
	return err == nil && value
}

func queryInt(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(c.Query(key)))
	if err != nil {
		return fallback
	}
	return value
}

func clampInt(value, min, max int) int {
	if max < min {
		max = min
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
