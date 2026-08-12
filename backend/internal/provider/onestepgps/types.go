package onestepgps

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"fleetview/internal/domain"
)


type deviceListResponse struct {
	ResultList []apiDevice `json:"result_list"`
}

type apiDevice struct {
	DeviceID    string     `json:"device_id"`
	DisplayName string     `json:"display_name"`
	FactoryID   string     `json:"factory_id"`
	Make        string     `json:"make"`
	Model       string     `json:"model"`
	ActiveState flexString `json:"active_state"`
	Online      flexBool   `json:"online"`

	LatestDevicePoint         *apiDevicePoint `json:"latest_device_point"`
	LatestAccurateDevicePoint *apiDevicePoint `json:"latest_accurate_device_point"`

	DeviceGroups apiGroups `json:"device_groups"`
}

type apiDevicePoint struct {
	Lat       flexFloat `json:"lat"`
	Lng       flexFloat `json:"lng"`
	Altitude  flexFloat `json:"altitude"`
	Angle     flexFloat `json:"angle"`
	Speed     flexFloat `json:"speed"`
	DtTracker flexTime  `json:"dt_tracker"`
	DtServer  flexTime  `json:"dt_server"`

	DeviceState *apiDeviceState `json:"device_state"`
}

type apiDeviceState struct {
	DriveStatus          flexString `json:"drive_status"`
	DriveStatusDuration  flexString `json:"drive_status_duration"`
	DriveStatusBeginTime flexTime   `json:"drive_status_begin_time"`
	Odometer             apiMeasure `json:"odometer"`
	SoftwareOdometer     apiMeasure `json:"software_odometer"`
}


func (d apiDevice) toDomain(speedUnit string, now time.Time) domain.Device {
	device := domain.Device{
		ID:        strings.TrimSpace(d.DeviceID),
		Name:      d.resolveName(),
		Make:      strings.TrimSpace(d.Make),
		Model:     strings.TrimSpace(d.Model),
		FactoryID: strings.TrimSpace(d.FactoryID),
		Online:    bool(d.Online),
		Active:    isActiveState(string(d.ActiveState)),
		Groups:    []string(d.DeviceGroups),
	}
	if device.Groups == nil {
		device.Groups = []string{}
	}

	point := d.bestPoint()
	if point == nil {
		device.DriveStatus = domain.DriveStatusUnknown
		return device
	}

	device.Position = domain.Position{
		Lat:        float64(point.Lat),
		Lng:        float64(point.Lng),
		Altitude:   float64(point.Altitude),
		Heading:    normaliseHeading(float64(point.Angle)),
		Speed:      float64(point.Speed),
		SpeedUnit:  speedUnit,
		RecordedAt: point.timestamp(now),
	}

	device.DriveStatus = domain.DriveStatusUnknown
	if state := point.DeviceState; state != nil {
		device.DriveStatus = domain.ParseDriveStatus(string(state.DriveStatus))
		device.DriveStatusDuration = parseISODuration(string(state.DriveStatusDuration))
		device.DriveStatusSince = state.DriveStatusBeginTime.Time
		odo := state.Odometer
		if odo.Value == 0 {
			odo = state.SoftwareOdometer
		}
		device.Odometer = odo.Value
		device.OdometerUnit = odo.Unit
	}

	
	if device.DriveStatus == domain.DriveStatusUnknown && device.Position.Speed > 1 {
		device.DriveStatus = domain.DriveStatusDriving
	}
	return device
}


func (d apiDevice) bestPoint() *apiDevicePoint {
	latest := d.LatestDevicePoint
	if latest != nil && latest.hasFix() {
		return latest
	}
	if d.LatestAccurateDevicePoint != nil && d.LatestAccurateDevicePoint.hasFix() {
		return d.LatestAccurateDevicePoint
	}
	return latest
}

func (d apiDevice) resolveName() string {
	if name := strings.TrimSpace(d.DisplayName); name != "" {
		return name
	}
	if combined := strings.TrimSpace(strings.TrimSpace(d.Make) + " " + strings.TrimSpace(d.Model)); combined != "" {
		return combined
	}
	if id := strings.TrimSpace(d.DeviceID); id != "" {
		return "Device " + id
	}
	return "Unnamed device"
}

func (p *apiDevicePoint) hasFix() bool {
	if p == nil {
		return false
	}
	return domain.Position{Lat: float64(p.Lat), Lng: float64(p.Lng)}.Valid()
}


func (p *apiDevicePoint) timestamp(now time.Time) time.Time {
	if !p.DtTracker.IsZero() {
		return p.DtTracker.Time
	}
	if !p.DtServer.IsZero() {
		return p.DtServer.Time
	}
	return now
}

func isActiveState(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "active", "true", "1":
		return true
	default:
		return false
	}
}

func normaliseHeading(angle float64) float64 {
	for angle < 0 {
		angle += 360
	}
	for angle >= 360 {
		angle -= 360
	}
	return angle
}


type flexTime struct{ time.Time }

var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func (t *flexTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "" || s == "null" {
		return nil
	}
	for _, layout := range timeLayouts {
		if parsed, err := time.Parse(layout, s); err == nil {
			t.Time = parsed
			return nil
		}
	}

	if secs, err := strconv.ParseInt(s, 10, 64); err == nil && secs > 0 {
		t.Time = time.Unix(secs, 0).UTC()
	}
	return nil
}


type flexFloat float64

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "" || s == "null" {
		return nil
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		*f = flexFloat(v)
	}
	return nil
}


type flexBool bool

func (b *flexBool) UnmarshalJSON(data []byte) error {
	s := strings.Trim(strings.TrimSpace(string(data)), `"`)
	switch strings.ToLower(s) {
	case "true", "1", "yes", "online":
		*b = true
	case "", "null":
		
	default:
		*b = false
	}
	return nil
}


type flexString string

func (s *flexString) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	if trimmed[0] == '"' {
		var v string
		if err := json.Unmarshal(trimmed, &v); err == nil {
			*s = flexString(v)
		}
		return nil
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		return nil
	}
	*s = flexString(trimmed)
	return nil
}


type apiMeasure struct {
	Value float64
	Unit  string
}

func (m *apiMeasure) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil
	}
	if trimmed[0] == '{' {
		var obj struct {
			Value flexFloat `json:"value"`
			Unit  string    `json:"unit"`
		}
		if err := json.Unmarshal(trimmed, &obj); err == nil {
			m.Value = float64(obj.Value)
			m.Unit = obj.Unit
		}
		return nil
	}
	var v flexFloat
	_ = json.Unmarshal(trimmed, &v)
	m.Value = float64(v)
	return nil
}


type apiGroups []string

func (g *apiGroups) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		var s string
		if err := json.Unmarshal(item, &s); err == nil {
			if s != "" {
				out = append(out, s)
			}
			continue
		}
		var obj struct {
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
		}
		if err := json.Unmarshal(item, &obj); err == nil {
			switch {
			case obj.Name != "":
				out = append(out, obj.Name)
			case obj.DisplayName != "":
				out = append(out, obj.DisplayName)
			}
		}
	}
	*g = out
	return nil
}


func parseISODuration(raw string) time.Duration {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0
	}
	if !strings.HasPrefix(s, "P") && !strings.HasPrefix(s, "p") {
		if d, err := time.ParseDuration(s); err == nil {
			return d
		}
		if secs, err := strconv.ParseFloat(s, 64); err == nil {
			return time.Duration(secs * float64(time.Second))
		}
		return 0
	}

	s = strings.ToUpper(s[1:])
	datePart, timePart, _ := strings.Cut(s, "T")

	var total time.Duration
	accumulate := func(part string, units map[byte]time.Duration) {
		var number strings.Builder
		for i := 0; i < len(part); i++ {
			c := part[i]
			if (c >= '0' && c <= '9') || c == '.' {
				number.WriteByte(c)
				continue
			}
			unit, ok := units[c]
			if !ok || number.Len() == 0 {
				number.Reset()
				continue
			}
			value, err := strconv.ParseFloat(number.String(), 64)
			number.Reset()
			if err != nil {
				continue
			}
			total += time.Duration(value * float64(unit))
		}
	}

	accumulate(datePart, map[byte]time.Duration{
		'Y': 365 * 24 * time.Hour,
		'W': 7 * 24 * time.Hour,
		'D': 24 * time.Hour,
		
		'M': 30 * 24 * time.Hour,
	})
	accumulate(timePart, map[byte]time.Duration{
		'H': time.Hour,
		'M': time.Minute,
		'S': time.Second,
	})
	return total
}
