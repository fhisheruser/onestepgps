package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fleetview/internal/config"
	"fleetview/internal/domain"
	"fleetview/internal/provider/demo"
	"fleetview/internal/repository"
	"fleetview/internal/service"
	"fleetview/internal/transport/dto"
)

var fixedNow = time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)

type stubClock struct{}

func (stubClock) Now() time.Time { return fixedNow }

type harness struct {
	router *gin.Engine
	cache  *service.SnapshotCache
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := repository.Open(repository.Options{
		Path:        filepath.Join(t.TempDir(), "api.db"),
		AutoMigrate: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = repository.Close(db) })

	cfg := config.Config{
		Env:            "test",
		Version:        "test-1",
		AllowedOrigins: []string{"http://localhost:5173"},
		Maps:           config.Maps{APIKey: "browser-key", MapID: "MAP123"},
		Icons:          config.Icons{MaxBytes: 64 << 10, AllowedTypes: []string{"image/png", "image/gif"}},
		History:        config.History{MaxPointsPerQuery: 100},
		Poller:         config.Poller{Interval: 10 * time.Second},
		RateLimit:      config.RateLimit{Enabled: false},
		OneStepGPS:     config.OneStepGPS{DemoMode: true},
	}

	log := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	cache := service.NewSnapshotCache()

	deviceService := service.NewDeviceService(service.DeviceServiceDeps{
		Cache:            cache,
		Preferences:      repository.NewPreferenceRepository(db),
		History:          repository.NewHistoryRepository(db),
		Clock:            stubClock{},
		MaxHistoryPoints: 100,
	})
	preferenceService := service.NewPreferenceService(service.PreferenceServiceDeps{
		Repo:             repository.NewPreferenceRepository(db),
		Icons:            repository.NewIconRepository(db),
		Clock:            stubClock{},
		MaxIconBytes:     cfg.Icons.MaxBytes,
		AllowedIconTypes: cfg.Icons.AllowedTypes,
	})
	poller := service.NewPoller(service.PollerDeps{
		Provider: demo.New("km/h", stubClock{}),
		Cache:    cache,
		Logger:   log,
		Clock:    stubClock{},
		Interval: 10 * time.Second,
	})

	router := NewRouter(RouterDeps{
		Handlers: NewHandlers(HandlerDeps{
			Devices:     deviceService,
			Preferences: preferenceService,
			Poller:      poller,
			Config:      cfg,
			Logger:      log,
			Clock:       stubClock{},
		}),
		Config: cfg,
		Logger: log,
	})

	h := &harness{router: router, cache: cache}
	h.seed()
	return h
}

// seed publishes a small, predictable fleet into the cache.
func (h *harness) seed() {
	h.cache.Set(domain.Snapshot{
		FetchedAt: fixedNow.Add(-4 * time.Second),
		Devices: []domain.Device{
			{
				ID: "d1", Name: "Truck 04", Make: "Freightliner", Model: "M2 106",
				Online: true, Active: true, DriveStatus: domain.DriveStatusDriving,
				Groups:   []string{"Logistics"},
				Position: domain.Position{Lat: 32.7, Lng: -117.1, Speed: 55, SpeedUnit: "km/h", RecordedAt: fixedNow.Add(-10 * time.Second)},
			},
			{
				ID: "d2", Name: "Van 12", Make: "Ford", Model: "Transit",
				Online: false, Active: true, DriveStatus: domain.DriveStatusOff,
				Groups:   []string{"Service"},
				Position: domain.Position{Lat: 32.8, Lng: -117.2, SpeedUnit: "km/h", RecordedAt: fixedNow.Add(-time.Hour)},
			},
		},
	})
}

func (h *harness) do(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(payload)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set(headerUserID, "alice")

	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func decodeFeed(t *testing.T, rec *httptest.ResponseRecorder) dto.Feed {
	t.Helper()
	var feed dto.Feed
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &feed))
	return feed
}

// ---------------------------------------------------------------------------

func TestListDevices_ReturnsMergedFeed(t *testing.T) {
	h := newHarness(t)

	rec := h.do(t, http.MethodGet, "/api/v1/devices", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	feed := decodeFeed(t, rec)
	require.Len(t, feed.Devices, 2)
	assert.Equal(t, "Truck 04", feed.Devices[0].Name)
	assert.Equal(t, "truck", feed.Devices[0].Preferences.MarkerIcon)
	assert.Equal(t, domain.DefaultMarkerColor, feed.Devices[0].Preferences.MarkerColor)
	assert.Equal(t, 2, feed.Summary.Total)
	assert.Equal(t, 1, feed.Summary.Driving)
	assert.Equal(t, 1, feed.Summary.Offline)
	assert.InDelta(t, 4, feed.Meta.AgeSeconds, 0.001)
	assert.False(t, feed.Meta.Stale)
	assert.NotEmpty(t, rec.Header().Get("X-Request-Id"))
}

func TestListDevices_FiltersAndSorts(t *testing.T) {
	h := newHarness(t)

	rec := h.do(t, http.MethodGet, "/api/v1/devices?status=driving", nil)
	feed := decodeFeed(t, rec)
	require.Len(t, feed.Devices, 1)
	assert.Equal(t, "d1", feed.Devices[0].ID)

	rec = h.do(t, http.MethodGet, "/api/v1/devices?search=ford", nil)
	feed = decodeFeed(t, rec)
	require.Len(t, feed.Devices, 1)
	assert.Equal(t, "d2", feed.Devices[0].ID)

	rec = h.do(t, http.MethodGet, "/api/v1/devices?sort=name&dir=desc", nil)
	feed = decodeFeed(t, rec)
	require.Len(t, feed.Devices, 2)
	assert.Equal(t, "Van 12", feed.Devices[0].Name)

	// Unknown filter values are ignored rather than rejected.
	rec = h.do(t, http.MethodGet, "/api/v1/devices?sort=bogus&status=bogus", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, decodeFeed(t, rec).Devices, 2)
}

func TestPreferences_RenameHideAndSurfaceInFeed(t *testing.T) {
	h := newHarness(t)

	rec := h.do(t, http.MethodPut, "/api/v1/preferences/devices/d1", map[string]any{
		"displayName": "Harbor Hauler",
		"markerColor": "#2E7D32",
		"markerIcon":  "truck",
		"pinned":      true,
		"notes":       "Refrigerated trailer",
	})
	require.Equal(t, http.StatusOK, rec.Code)

	rec = h.do(t, http.MethodPut, "/api/v1/preferences/devices/d2", map[string]any{"hidden": true})
	require.Equal(t, http.StatusOK, rec.Code)

	feed := decodeFeed(t, h.do(t, http.MethodGet, "/api/v1/devices", nil))
	require.Len(t, feed.Devices, 1, "hidden devices drop out of the default feed")
	assert.Equal(t, "Harbor Hauler", feed.Devices[0].Name)
	assert.Equal(t, "Truck 04", feed.Devices[0].ProviderName)
	assert.True(t, feed.Devices[0].Renamed)
	assert.Equal(t, "#2E7D32", feed.Devices[0].Preferences.MarkerColor)
	assert.True(t, feed.Devices[0].Preferences.Pinned)
	assert.Equal(t, 1, feed.Summary.Hidden)

	withHidden := decodeFeed(t, h.do(t, http.MethodGet, "/api/v1/devices?includeHidden=true", nil))
	assert.Len(t, withHidden.Devices, 2)
}

func TestPreferences_AreScopedPerUser(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, http.StatusOK, h.do(t, http.MethodPut, "/api/v1/preferences/devices/d1",
		map[string]any{"displayName": "Alice's Truck"}).Code)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	req.Header.Set(headerUserID, "bob")
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)

	feed := decodeFeed(t, rec)
	require.Len(t, feed.Devices, 2)
	for _, d := range feed.Devices {
		assert.NotEqual(t, "Alice's Truck", d.Name)
	}
}

func TestPreferences_RejectsInvalidValues(t *testing.T) {
	h := newHarness(t)

	rec := h.do(t, http.MethodPut, "/api/v1/preferences/devices/d1", map[string]any{"markerColor": "puce"})
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	var envelope dto.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.NotNil(t, envelope.Error)
	assert.Equal(t, "validation_failed", envelope.Error.Code)
	assert.Equal(t, "markerColor", envelope.Error.Field)

	rec = h.do(t, http.MethodPut, "/api/v1/preferences/devices/d1", map[string]any{"markerIcon": "spaceship"})
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	rec = h.do(t, http.MethodPut, "/api/v1/preferences/devices/d1", map[string]any{"displayName": strings.Repeat("x", 200)})
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestSettings_UpdateAndNormalise(t *testing.T) {
	h := newHarness(t)

	rec := h.do(t, http.MethodPut, "/api/v1/preferences/settings", map[string]any{
		"theme":          "dark",
		"speedUnit":      "kph",
		"refreshSeconds": 5000,
		"showTrails":     true,
	})
	require.Equal(t, http.StatusOK, rec.Code)

	var settings dto.Settings
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &settings))
	assert.Equal(t, "dark", settings.Theme)
	assert.Equal(t, "kph", settings.SpeedUnit)
	assert.Equal(t, domain.MaxRefreshSeconds, settings.RefreshSeconds, "out-of-range values are clamped")
	assert.True(t, settings.ShowTrails)

	// The stored setting drives the feed's default ordering.
	rec = h.do(t, http.MethodPut, "/api/v1/preferences/settings", map[string]any{"sortKey": "name", "sortDirection": "desc"})
	require.Equal(t, http.StatusOK, rec.Code)
	feed := decodeFeed(t, h.do(t, http.MethodGet, "/api/v1/devices", nil))
	assert.Equal(t, "Van 12", feed.Devices[0].Name)
}

func TestSettings_HideOfflineDevices(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, http.StatusOK, h.do(t, http.MethodPut, "/api/v1/preferences/settings",
		map[string]any{"showOfflineDevices": false}).Code)

	feed := decodeFeed(t, h.do(t, http.MethodGet, "/api/v1/devices", nil))
	require.Len(t, feed.Devices, 1)
	assert.Equal(t, "d1", feed.Devices[0].ID)

	// An explicit filter still wins over the stored preference.
	feed = decodeFeed(t, h.do(t, http.MethodGet, "/api/v1/devices?status=all", nil))
	assert.Len(t, feed.Devices, 2)
}

func TestResetPreferences(t *testing.T) {
	h := newHarness(t)

	require.Equal(t, http.StatusOK, h.do(t, http.MethodPut, "/api/v1/preferences/devices/d1",
		map[string]any{"displayName": "Temporary"}).Code)
	require.Equal(t, http.StatusNoContent, h.do(t, http.MethodPost, "/api/v1/preferences/reset", nil).Code)

	feed := decodeFeed(t, h.do(t, http.MethodGet, "/api/v1/devices", nil))
	assert.Equal(t, "Truck 04", feed.Devices[0].Name)
}

func TestReorderDevices_SwitchesToCustomSort(t *testing.T) {
	h := newHarness(t)

	rec := h.do(t, http.MethodPost, "/api/v1/preferences/reorder", map[string]any{"deviceIds": []string{"d2", "d1"}})
	require.Equal(t, http.StatusNoContent, rec.Code)

	feed := decodeFeed(t, h.do(t, http.MethodGet, "/api/v1/devices", nil))
	require.Len(t, feed.Devices, 2)
	assert.Equal(t, "d2", feed.Devices[0].ID)
	assert.Equal(t, "custom", feed.Settings.SortKey)

	assert.Equal(t, http.StatusUnprocessableEntity,
		h.do(t, http.MethodPost, "/api/v1/preferences/reorder", map[string]any{"deviceIds": []string{}}).Code)
}

func TestGetDevice_And404(t *testing.T) {
	h := newHarness(t)

	rec := h.do(t, http.MethodGet, "/api/v1/devices/d1", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var device dto.Device
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &device))
	assert.Equal(t, "d1", device.ID)
	assert.True(t, device.Position.Valid)

	rec = h.do(t, http.MethodGet, "/api/v1/devices/does-not-exist", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestExportCSV(t *testing.T) {
	h := newHarness(t)

	rec := h.do(t, http.MethodGet, "/api/v1/export/devices.csv", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/csv")
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "attachment")

	body := rec.Body.String()
	lines := strings.Split(strings.TrimSpace(body), "\n")
	require.Len(t, lines, 3, "header plus one row per device")
	assert.Contains(t, lines[0], "device_id")
	assert.Contains(t, body, "Truck 04")
}

func TestIconUpload_AcceptsPNGAndRejectsJunk(t *testing.T) {
	h := newHarness(t)

	// A minimal but real 1x1 PNG.
	png := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00,
		0x0A, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49,
		0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
	}

	rec := h.upload(t, "/api/v1/preferences/devices/d1/icon", "marker.png", "image/png", png)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var pref dto.DevicePreference
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &pref))
	assert.Equal(t, "custom", pref.MarkerIcon)
	require.True(t, strings.HasPrefix(pref.CustomIconURL, "/api/v1/icons/"))

	// The uploaded image is served back with hardening headers.
	iconRec := h.do(t, http.MethodGet, pref.CustomIconURL, nil)
	require.Equal(t, http.StatusOK, iconRec.Code)
	assert.Equal(t, "image/png", iconRec.Header().Get("Content-Type"))
	assert.Equal(t, "nosniff", iconRec.Header().Get("X-Content-Type-Options"))
	assert.Contains(t, iconRec.Header().Get("Content-Security-Policy"), "default-src 'none'")

	// The feed now points at the custom marker.
	feed := decodeFeed(t, h.do(t, http.MethodGet, "/api/v1/devices", nil))
	assert.Equal(t, "custom", feed.Devices[0].Preferences.MarkerIcon)

	// A file that merely claims to be a PNG is rejected on its bytes.
	rec = h.upload(t, "/api/v1/preferences/devices/d2/icon", "evil.png", "image/png",
		[]byte("<svg xmlns='http://www.w3.org/2000/svg'><script>alert(1)</script></svg>"))
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	// Removing the icon reverts to a built-in marker.
	rec = h.do(t, http.MethodDelete, "/api/v1/preferences/devices/d1/icon", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	feed = decodeFeed(t, h.do(t, http.MethodGet, "/api/v1/devices", nil))
	assert.NotEqual(t, "custom", feed.Devices[0].Preferences.MarkerIcon)
}

func (h *harness) upload(t *testing.T, path, filename, contentType string, data []byte) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(map[string][]string)
	header["Content-Disposition"] = []string{`form-data; name="icon"; filename="` + filename + `"`}
	header["Content-Type"] = []string{contentType}
	part, err := writer.CreatePart(header)
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set(headerUserID, "alice")

	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func TestRuntimeConfig_NeverLeaksTheProviderKey(t *testing.T) {
	h := newHarness(t)

	rec := h.do(t, http.MethodGet, "/api/v1/config", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var cfg dto.RuntimeConfig
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &cfg))
	assert.Equal(t, "browser-key", cfg.GoogleMapsAPIKey)
	assert.Equal(t, "MAP123", cfg.GoogleMapsMapID)
	assert.True(t, cfg.DemoMode)

	body := strings.ToLower(rec.Body.String())
	assert.NotContains(t, body, "onestep")
	assert.NotContains(t, body, "api-key")
}

func TestHealthAndReadiness(t *testing.T) {
	h := newHarness(t)

	rec := h.do(t, http.MethodGet, "/healthz", nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = h.do(t, http.MethodGet, "/readyz", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"ready"`)

	// Before the first snapshot lands the service is not ready.
	empty := newHarness(t)
	empty.cache.Set(domain.Snapshot{})
	rec = empty.do(t, http.MethodGet, "/readyz", nil)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"starting"`)
}

func TestStaleSnapshotIsReportedNotHidden(t *testing.T) {
	h := newHarness(t)
	h.cache.MarkStale("provider timeout")

	feed := decodeFeed(t, h.do(t, http.MethodGet, "/api/v1/devices", nil))
	assert.True(t, feed.Meta.Stale)
	assert.Equal(t, "provider timeout", feed.Meta.Error)
	assert.Len(t, feed.Devices, 2, "stale data still beats no data")
}

func TestCORSAndUnknownRoutes(t *testing.T) {
	h := newHarness(t)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/devices", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "http://localhost:5173", rec.Header().Get("Access-Control-Allow-Origin"))

	req = httptest.NewRequest(http.MethodOptions, "/api/v1/devices", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"), "untrusted origins get no CORS grant")

	rec = h.do(t, http.MethodGet, "/api/v1/nope", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "not_found")

	rec = h.do(t, http.MethodDelete, "/api/v1/devices", nil)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestUserIDSanitisation(t *testing.T) {
	h := newHarness(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices", nil)
	req.Header.Set(headerUserID, "../../etc/passwd; DROP TABLE users")
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code, "a hostile id falls back to the default scope")
}
