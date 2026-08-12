package onestepgps

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fleetview/internal/domain"
)

// samplePayload mirrors the shape of a real OneStepGPS response, including the
// awkward parts: a device with no fix, a numeric-string speed, a null
// device_state and an unknown extra field.
const samplePayload = `{
  "result_list": [
    {
      "device_id": "abc-123",
      "display_name": "Truck 04",
      "factory_id": "F-9001",
      "make": "Freightliner",
      "model": "M2 106",
      "active_state": "active",
      "online": true,
      "device_groups": [{"device_group_id": "g1", "name": "Logistics"}],
      "latest_device_point": {
        "lat": 32.7157,
        "lng": -117.1611,
        "altitude": 12.5,
        "angle": 271.4,
        "speed": "48.3",
        "dt_tracker": "2026-08-11T10:15:30.000Z",
        "dt_server": "2026-08-11T10:15:31.000Z",
        "device_state": {
          "drive_status": "driving",
          "drive_status_duration": "PT1H22M15S",
          "drive_status_begin_time": "2026-08-11T08:53:15Z",
          "odometer": {"value": 184320.5, "unit": "km"}
        },
        "unexpected_future_field": {"nested": true}
      }
    },
    {
      "device_id": "def-456",
      "display_name": "Van 12",
      "active_state": "inactive",
      "online": false,
      "device_groups": ["Service"],
      "latest_device_point": {
        "lat": 0,
        "lng": 0,
        "speed": 0,
        "dt_tracker": null,
        "device_state": null
      },
      "latest_accurate_device_point": {
        "lat": 32.742,
        "lng": -117.13,
        "speed": 0,
        "dt_tracker": "2026-08-11T09:00:00Z",
        "device_state": {"drive_status": "off", "odometer": 97650}
      }
    },
    {
      "device_id": "",
      "display_name": "Broken device without an id"
    }
  ]
}`

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestFetchDevices_ParsesUpstreamPayload(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, samplePayload)
	}))
	defer server.Close()

	client := New(Options{
		BaseURL:   server.URL,
		APIKey:    "secret-key",
		SpeedUnit: "km/h",
	}, testLogger())

	devices, err := client.FetchDevices(context.Background())
	require.NoError(t, err)

	// The credentials and the latest_point flag must be on the wire.
	assert.Contains(t, gotQuery, "api-key=secret-key")
	assert.Contains(t, gotQuery, "latest_point=true")

	// The device without an id is dropped rather than poisoning the fleet.
	require.Len(t, devices, 2)

	truck := devices[0]
	assert.Equal(t, "abc-123", truck.ID)
	assert.Equal(t, "Truck 04", truck.Name)
	assert.True(t, truck.Online)
	assert.True(t, truck.Active)
	assert.Equal(t, domain.DriveStatusDriving, truck.DriveStatus)
	assert.InDelta(t, 48.3, truck.Position.Speed, 0.001, "numeric strings must parse")
	assert.InDelta(t, 32.7157, truck.Position.Lat, 0.00001)
	assert.Equal(t, "km/h", truck.Position.SpeedUnit)
	assert.Equal(t, time.Hour+22*time.Minute+15*time.Second, truck.DriveStatusDuration)
	assert.InDelta(t, 184320.5, truck.Odometer, 0.01)
	assert.Equal(t, []string{"Logistics"}, truck.Groups)

	van := devices[1]
	assert.False(t, van.Active, "active_state=inactive must map to Active=false")
	// (0,0) is not a real fix, so the accurate point wins.
	assert.InDelta(t, 32.742, van.Position.Lat, 0.00001)
	assert.Equal(t, domain.DriveStatusOff, van.DriveStatus)
	assert.InDelta(t, 97650, van.Odometer, 0.01, "a bare numeric odometer must parse")
	assert.Equal(t, []string{"Service"}, van.Groups)
}

func TestFetchDevices_RetriesTransientFailures(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = io.WriteString(w, `{"result_list":[{"device_id":"x","display_name":"X"}]}`)
	}))
	defer server.Close()

	client := New(Options{
		BaseURL:      server.URL,
		APIKey:       "k",
		MaxAttempts:  3,
		RetryBackoff: time.Millisecond,
	}, testLogger())

	devices, err := client.FetchDevices(context.Background())
	require.NoError(t, err)
	require.Len(t, devices, 1)
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls))
}

func TestFetchDevices_GivesUpAfterMaxAttempts(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := New(Options{BaseURL: server.URL, MaxAttempts: 2, RetryBackoff: time.Millisecond}, testLogger())

	_, err := client.FetchDevices(context.Background())
	require.Error(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls))
	assert.Contains(t, err.Error(), "after 2 attempt(s)")
}

func TestFetchDevices_DoesNotRetryAuthFailures(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"bad api key"}`)
	}))
	defer server.Close()

	client := New(Options{BaseURL: server.URL, APIKey: "wrong", MaxAttempts: 3, RetryBackoff: time.Millisecond}, testLogger())

	_, err := client.FetchDevices(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrUpstreamAuth), "auth failures must be distinguishable")
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "retrying a rejected key just burns quota")
}

func TestFetchDevices_RejectsMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"result_list": [`)
	}))
	defer server.Close()

	client := New(Options{BaseURL: server.URL, MaxAttempts: 1}, testLogger())

	_, err := client.FetchDevices(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode response")
}

func TestFetchDevices_HonoursContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	client := New(Options{BaseURL: server.URL, MaxAttempts: 3, RetryBackoff: time.Millisecond}, testLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.FetchDevices(ctx)
	require.Error(t, err)
}

func TestRedaction_KeepsAPIKeyOutOfErrors(t *testing.T) {
	const key = "super-secret-key"

	err := redactError(errors.New(`Get "https://x/?api-key=`+key+`": dial failed`), key)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), key)
	assert.Contains(t, err.Error(), "REDACTED")

	assert.False(t, strings.Contains(redactURL("https://track.example/v3/device?api-key="+key), key))
}

func TestBackoffFor_GrowsAndStaysBounded(t *testing.T) {
	client := New(Options{BaseURL: "https://example.test", RetryBackoff: time.Second}, testLogger())

	for attempt := 1; attempt <= 8; attempt++ {
		delay := client.backoffFor(attempt)
		assert.Greater(t, delay, time.Duration(0))
		assert.LessOrEqual(t, delay, 10*time.Second, "backoff must stay capped")
	}
}
