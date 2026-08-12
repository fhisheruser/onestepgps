package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// SortKey identifies how a device list should be ordered.
type SortKey string

const (
	SortKeyName    SortKey = "name"
	SortKeyStatus  SortKey = "status"
	SortKeySpeed   SortKey = "speed"
	SortKeyUpdated SortKey = "updated"
	SortKeyCustom  SortKey = "custom"
)

// SortDirection is the ordering direction applied to a SortKey.
type SortDirection string

const (
	SortAsc  SortDirection = "asc"
	SortDesc SortDirection = "desc"
)

// SpeedUnit is the unit the UI renders speeds in.
type SpeedUnit string

const (
	SpeedMPH SpeedUnit = "mph"
	SpeedKPH SpeedUnit = "kph"
	SpeedKn  SpeedUnit = "kn"
)

// Theme is the colour scheme preference.
type Theme string

const (
	ThemeLight  Theme = "light"
	ThemeDark   Theme = "dark"
	ThemeSystem Theme = "system"
)

// MarkerIcon is the vehicle silhouette drawn for a device on the map.
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

// Default marker styling applied to devices the user has never customised.
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

// UserSettings holds the fleet-wide personalisation for one user.
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

// DefaultUserSettings returns the settings a brand new user starts with.
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

// Normalize coerces out-of-range or unknown values back to sane defaults so a
// hand-crafted request can never poison the stored settings.
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

// DevicePreference holds the per-device personalisation for one user.
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

// IsZeroValue reports whether the preference carries no customisation, in which
// case the repository can drop the row instead of storing dead weight.
func (p DevicePreference) IsZeroValue() bool {
	return !p.Hidden && !p.Pinned &&
		p.DisplayName == "" && p.Notes == "" &&
		p.CustomIconURL == "" && p.SortIndex == 0 &&
		(p.MarkerIcon == "" || p.MarkerIcon == IconCar) &&
		(p.MarkerColor == "" || p.MarkerColor == DefaultMarkerColor)
}

// Validate enforces the invariants of a device preference.
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

// Icon is a user-uploaded marker image stored in the database.
type Icon struct {
	ID          string
	UserID      string
	DeviceID    string
	ContentType string
	Size        int
	Data        []byte
	CreatedAt   time.Time
}
