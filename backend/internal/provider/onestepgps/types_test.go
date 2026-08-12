package onestepgps

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fleetview/internal/domain"
)

func TestParseISODuration(t *testing.T) {
	cases := map[string]time.Duration{
		"PT10H":       10 * time.Hour,
		"PT1H22M15S":  time.Hour + 22*time.Minute + 15*time.Second,
		"P1DT2H30M":   26*time.Hour + 30*time.Minute,
		"PT0S":        0,
		"PT45.5S":     45*time.Second + 500*time.Millisecond,
		"":            0,
		"not-a-value": 0,
		"90s":         90 * time.Second,
		"120":         120 * time.Second,
	}

	for input, expected := range cases {
		t.Run(input, func(t *testing.T) {
			assert.Equal(t, expected, parseISODuration(input))
		})
	}
}

func TestParseDriveStatus(t *testing.T) {
	assert.Equal(t, domain.DriveStatusDriving, domain.ParseDriveStatus("Driving"))
	assert.Equal(t, domain.DriveStatusIdle, domain.ParseDriveStatus(" idle "))
	assert.Equal(t, domain.DriveStatusOff, domain.ParseDriveStatus("OFF"))
	assert.Equal(t, domain.DriveStatusUnknown, domain.ParseDriveStatus("teleporting"))
	assert.Equal(t, domain.DriveStatusUnknown, domain.ParseDriveStatus(""))
}

func TestFlexScalars_TolerateWireVariations(t *testing.T) {
	var payload struct {
		A flexFloat  `json:"a"`
		B flexFloat  `json:"b"`
		C flexBool   `json:"c"`
		D flexBool   `json:"d"`
		E flexString `json:"e"`
		F flexString `json:"f"`
		G flexTime   `json:"g"`
		H flexTime   `json:"h"`
	}

	raw := `{"a": 12.5, "b": "12.5", "c": true, "d": "true",
	         "e": "text", "f": {"unexpected": "object"},
	         "g": "2026-08-11T10:15:30Z", "h": "garbage"}`
	require.NoError(t, json.Unmarshal([]byte(raw), &payload))

	assert.InDelta(t, 12.5, float64(payload.A), 0.0001)
	assert.InDelta(t, 12.5, float64(payload.B), 0.0001)
	assert.True(t, bool(payload.C))
	assert.True(t, bool(payload.D))
	assert.Equal(t, flexString("text"), payload.E)
	assert.Equal(t, flexString(""), payload.F, "an object where a string was expected must not fail the parse")
	assert.Equal(t, 2026, payload.G.Year())
	assert.True(t, payload.H.IsZero(), "an unparseable timestamp degrades to zero")
}

func TestApiMeasure_AcceptsObjectOrNumber(t *testing.T) {
	var payload struct {
		Object apiMeasure `json:"object"`
		Number apiMeasure `json:"number"`
		Null   apiMeasure `json:"null"`
	}
	raw := `{"object": {"value": 100.5, "unit": "km"}, "number": 42, "null": null}`
	require.NoError(t, json.Unmarshal([]byte(raw), &payload))

	assert.InDelta(t, 100.5, payload.Object.Value, 0.001)
	assert.Equal(t, "km", payload.Object.Unit)
	assert.InDelta(t, 42, payload.Number.Value, 0.001)
	assert.InDelta(t, 0, payload.Null.Value, 0.001)
}

func TestToDomain_FallsBackForNameAndTimestamp(t *testing.T) {
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)

	device := apiDevice{
		DeviceID: "id-1",
		Make:     "Ford",
		Model:    "Transit",
		LatestDevicePoint: &apiDevicePoint{
			Lat: 32.7, Lng: -117.1,
		},
	}.toDomain("mph", now)

	assert.Equal(t, "Ford Transit", device.Name, "display_name is optional")
	assert.Equal(t, now, device.Position.RecordedAt, "a missing timestamp falls back to now")
	assert.Equal(t, domain.DriveStatusUnknown, device.DriveStatus)
	assert.True(t, device.Active, "a missing active_state is treated as active")
}

func TestToDomain_InfersMovementWhenStatusMissing(t *testing.T) {
	device := apiDevice{
		DeviceID: "id-2",
		LatestDevicePoint: &apiDevicePoint{
			Lat: 32.7, Lng: -117.1, Speed: 42,
			DeviceState: &apiDeviceState{DriveStatus: ""},
		},
	}.toDomain("mph", time.Now())

	assert.Equal(t, domain.DriveStatusDriving, device.DriveStatus)
}

func TestPositionValid(t *testing.T) {
	assert.True(t, domain.Position{Lat: 32.7, Lng: -117.1}.Valid())
	assert.False(t, domain.Position{Lat: 0, Lng: 0}.Valid(), "a null island fix is not a real fix")
	assert.False(t, domain.Position{Lat: 91, Lng: 0}.Valid())
	assert.False(t, domain.Position{Lat: 10, Lng: 181}.Valid())
}
