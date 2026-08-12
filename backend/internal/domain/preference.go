package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)


type SortKey string

const (
	SortKeyName    SortKey = "name"
	SortKeyStatus  SortKey = "status"
	SortKeySpeed   SortKey = "speed"
	SortKeyUpdated SortKey = "updated"
	SortKeyCustom  SortKey = "custom"
)

type SortDirection string

const (
	SortAsc  SortDirection = "asc"
	SortDesc SortDirection = "desc"
)


type SpeedUnit string

const (
	SpeedMPH SpeedUnit = "mph"
	SpeedKPH SpeedUnit = "kph"
	SpeedKn  SpeedUnit = "kn"
)

type Theme string

const (
	ThemeLight  Theme = "light"
	ThemeDark   Theme = "dark"
	ThemeSystem Theme = "system"
)


type MarkerIcon string

const (
	IconCar    MarkerIcon = "car"
	IconTruck  MarkerIcon = "truck"
	IconVan    MarkerIcon = "van"
	IconBus    MarkerIcon = "bus"
	IconPickup MarkerIcon = "pickup"
	IconPin    MarkerIcon = "pin"
	IconCustom MarkerIcon = "custom"
)


const (
	DefaultMarkerColor = "#B4643C"
	MinRefreshSeconds  = 5
	MaxRefreshSeconds  = 300
	MaxDisplayNameLen  = 64
	MaxNotesLen        = 500
)

var (
	hexColorRe   = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
	validIcons   = map[MarkerIcon]bool{IconCar: true, IconTruck: true, IconVan: true, IconBus: true, IconPickup: true, IconPin: true, IconCustom: true}
	validSortKey = map[SortKey]bool{SortKeyName: true, SortKeyStatus: true, SortKeySpeed: true, SortKeyUpdated: true, SortKeyCustom: true}
	validUnits   = map[SpeedUnit]bool{SpeedMPH: true, SpeedKPH: true, SpeedKn: true}
	validThemes  = map[Theme]bool{ThemeLight: true, ThemeDark: true, ThemeSystem: true}
	validMaps    = map[string]bool{"roadmap": true, "satellite": true, "hybrid": true, "terrain": true}
)


type UserSettings struct {
	UserID             string
	Theme              Theme
	SortKey            SortKey
	SortDirection      SortDirection
	SpeedUnit          SpeedUnit
	MapType            string
	ShowOfflineDevices bool
	ClusterMarkers     bool
	ShowTrails         bool
	AnimateMarkers     bool
	AutoFitBounds      bool
	RefreshSeconds     int
	UpdatedAt          time.Time
}


func DefaultUserSettings(userID string) UserSettings {
	return UserSettings{
		UserID:             userID,
		Theme:              ThemeSystem,
		SortKey:            SortKeyName,
		SortDirection:      SortAsc,
		SpeedUnit:          SpeedMPH,
		MapType:            "roadmap",
		ShowOfflineDevices: true,
		ClusterMarkers:     true,
		ShowTrails:         false,
		AnimateMarkers:     true,
		AutoFitBounds:      true,
		RefreshSeconds:     10,
	}
}


func (s *UserSettings) Normalize() {
	def := DefaultUserSettings(s.UserID)
	if !validThemes[s.Theme] {
		s.Theme = def.Theme
	}
	if !validSortKey[s.SortKey] {
		s.SortKey = def.SortKey
	}
	if s.SortDirection != SortAsc && s.SortDirection != SortDesc {
		s.SortDirection = def.SortDirection
	}
	if !validUnits[s.SpeedUnit] {
		s.SpeedUnit = def.SpeedUnit
	}
	if !validMaps[strings.ToLower(s.MapType)] {
		s.MapType = def.MapType
	} else {
		s.MapType = strings.ToLower(s.MapType)
	}
	if s.RefreshSeconds < MinRefreshSeconds {
		s.RefreshSeconds = MinRefreshSeconds
	}
	if s.RefreshSeconds > MaxRefreshSeconds {
		s.RefreshSeconds = MaxRefreshSeconds
	}
}


type DevicePreference struct {
	UserID        string
	DeviceID      string
	Hidden        bool
	DisplayName   string
	MarkerIcon    MarkerIcon
	MarkerColor   string
	CustomIconURL string
	Pinned        bool
	SortIndex     int
	Notes         string
	UpdatedAt     time.Time
}


func (p DevicePreference) IsZeroValue() bool {
	return !p.Hidden && !p.Pinned &&
		p.DisplayName == "" && p.Notes == "" &&
		p.CustomIconURL == "" && p.SortIndex == 0 &&
		(p.MarkerIcon == "" || p.MarkerIcon == IconCar) &&
		(p.MarkerColor == "" || p.MarkerColor == DefaultMarkerColor)
}


func (p *DevicePreference) Validate() error {
	if strings.TrimSpace(p.DeviceID) == "" {
		return NewValidationError("deviceId", "device id is required")
	}
	p.DisplayName = strings.TrimSpace(p.DisplayName)
	if len([]rune(p.DisplayName)) > MaxDisplayNameLen {
		return NewValidationError("displayName", fmt.Sprintf("must be at most %d characters", MaxDisplayNameLen))
	}
	p.Notes = strings.TrimSpace(p.Notes)
	if len([]rune(p.Notes)) > MaxNotesLen {
		return NewValidationError("notes", fmt.Sprintf("must be at most %d characters", MaxNotesLen))
	}
	if p.MarkerColor == "" {
		p.MarkerColor = DefaultMarkerColor
	}
	if !hexColorRe.MatchString(p.MarkerColor) {
		return NewValidationError("markerColor", "must be a hex colour such as #B4643C")
	}
	if p.MarkerIcon == "" {
		p.MarkerIcon = IconCar
	}
	if !validIcons[p.MarkerIcon] {
		return NewValidationError("markerIcon", "unsupported marker icon")
	}
	if p.MarkerIcon == IconCustom && p.CustomIconURL == "" {
		return NewValidationError("markerIcon", "upload an icon before selecting the custom marker")
	}
	return nil
}


type Icon struct {
	ID          string
	UserID      string
	DeviceID    string
	ContentType string
	Size        int
	Data        []byte
	CreatedAt   time.Time
}
