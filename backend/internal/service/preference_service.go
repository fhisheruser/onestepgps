package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"fleetview/internal/domain"
)

// SettingsPatch is a partial update of fleet-wide settings. Pointer fields
// distinguish "not supplied" from "set to the zero value", which is what makes
// a boolean like ShowOfflineDevices safely togglable.
type SettingsPatch struct {
	Theme              *string
	SortKey            *string
	SortDirection      *string
	SpeedUnit          *string
	MapType            *string
	ShowOfflineDevices *bool
	ClusterMarkers     *bool
	ShowTrails         *bool
	AnimateMarkers     *bool
	AutoFitBounds      *bool
	RefreshSeconds     *int
}

// DevicePreferencePatch is a partial update of one device's personalisation.
type DevicePreferencePatch struct {
	Hidden      *bool
	DisplayName *string
	MarkerIcon  *string
	MarkerColor *string
	Pinned      *bool
	SortIndex   *int
	Notes       *string
}

// Preferences bundles everything the preferences endpoint returns.
type Preferences struct {
	Settings domain.UserSettings
	Devices  []domain.DevicePreference
}

// PreferenceServiceDeps are the collaborators of PreferenceService.
type PreferenceServiceDeps struct {
	Repo             domain.PreferenceRepository
	Icons            domain.IconRepository
	Clock            domain.Clock
	MaxIconBytes     int64
	AllowedIconTypes []string
	IconURLPrefix    string
}

// PreferenceService is the write side of the application: it validates and
// persists everything the user personalises.
type PreferenceService struct {
	repo          domain.PreferenceRepository
	icons         domain.IconRepository
	clock         domain.Clock
	maxIconBytes  int64
	allowedTypes  map[string]bool
	iconURLPrefix string
}

// NewPreferenceService wires a PreferenceService.
func NewPreferenceService(deps PreferenceServiceDeps) *PreferenceService {
	if deps.Clock == nil {
		deps.Clock = domain.SystemClock{}
	}
	if deps.MaxIconBytes <= 0 {
		deps.MaxIconBytes = 256 << 10
	}
	if deps.IconURLPrefix == "" {
		deps.IconURLPrefix = "/api/v1/icons/"
	}
	allowed := make(map[string]bool, len(deps.AllowedIconTypes))
	for _, t := range deps.AllowedIconTypes {
		allowed[strings.ToLower(strings.TrimSpace(t))] = true
	}
	if len(allowed) == 0 {
		allowed = map[string]bool{"image/png": true, "image/jpeg": true, "image/webp": true, "image/gif": true}
	}

	return &PreferenceService{
		repo:          deps.Repo,
		icons:         deps.Icons,
		clock:         deps.Clock,
		maxIconBytes:  deps.MaxIconBytes,
		allowedTypes:  allowed,
		iconURLPrefix: deps.IconURLPrefix,
	}
}

// Get returns the full preference document for a user.
func (s *PreferenceService) Get(ctx context.Context, userID string) (Preferences, error) {
	settings, err := s.repo.GetSettings(ctx, userID)
	if err != nil {
		return Preferences{}, fmt.Errorf("load settings: %w", err)
	}
	devices, err := s.repo.ListDevicePreferences(ctx, userID)
	if err != nil {
		return Preferences{}, fmt.Errorf("load device preferences: %w", err)
	}
	return Preferences{Settings: settings, Devices: devices}, nil
}

// UpdateSettings applies a patch to the user's fleet-wide settings.
func (s *PreferenceService) UpdateSettings(ctx context.Context, userID string, patch SettingsPatch) (domain.UserSettings, error) {
	settings, err := s.repo.GetSettings(ctx, userID)
	if err != nil {
		return domain.UserSettings{}, fmt.Errorf("load settings: %w", err)
	}

	if patch.Theme != nil {
		settings.Theme = domain.Theme(strings.ToLower(*patch.Theme))
	}
	if patch.SortKey != nil {
		settings.SortKey = domain.SortKey(strings.ToLower(*patch.SortKey))
	}
	if patch.SortDirection != nil {
		settings.SortDirection = domain.SortDirection(strings.ToLower(*patch.SortDirection))
	}
	if patch.SpeedUnit != nil {
		settings.SpeedUnit = domain.SpeedUnit(strings.ToLower(*patch.SpeedUnit))
	}
	if patch.MapType != nil {
		settings.MapType = strings.ToLower(*patch.MapType)
	}
	if patch.ShowOfflineDevices != nil {
		settings.ShowOfflineDevices = *patch.ShowOfflineDevices
	}
	if patch.ClusterMarkers != nil {
		settings.ClusterMarkers = *patch.ClusterMarkers
	}
	if patch.ShowTrails != nil {
		settings.ShowTrails = *patch.ShowTrails
	}
	if patch.AnimateMarkers != nil {
		settings.AnimateMarkers = *patch.AnimateMarkers
	}
	if patch.AutoFitBounds != nil {
		settings.AutoFitBounds = *patch.AutoFitBounds
	}
	if patch.RefreshSeconds != nil {
		settings.RefreshSeconds = *patch.RefreshSeconds
	}

	settings.UserID = userID
	settings.Normalize()
	settings.UpdatedAt = s.clock.Now()

	if err := s.repo.SaveSettings(ctx, settings); err != nil {
		return domain.UserSettings{}, fmt.Errorf("save settings: %w", err)
	}
	return settings, nil
}

// UpsertDevicePreference applies a patch to one device's personalisation.
func (s *PreferenceService) UpsertDevicePreference(ctx context.Context, userID, deviceID string, patch DevicePreferencePatch) (domain.DevicePreference, error) {
	pref, err := s.repo.GetDevicePreference(ctx, userID, deviceID)
	if err != nil {
		return domain.DevicePreference{}, fmt.Errorf("load device preference: %w", err)
	}
	pref.UserID = userID
	pref.DeviceID = deviceID

	if patch.Hidden != nil {
		pref.Hidden = *patch.Hidden
	}
	if patch.DisplayName != nil {
		pref.DisplayName = *patch.DisplayName
	}
	if patch.MarkerIcon != nil {
		pref.MarkerIcon = domain.MarkerIcon(strings.ToLower(*patch.MarkerIcon))
	}
	if patch.MarkerColor != nil {
		pref.MarkerColor = strings.TrimSpace(*patch.MarkerColor)
	}
	if patch.Pinned != nil {
		pref.Pinned = *patch.Pinned
	}
	if patch.SortIndex != nil {
		pref.SortIndex = *patch.SortIndex
	}
	if patch.Notes != nil {
		pref.Notes = *patch.Notes
	}

	if err := pref.Validate(); err != nil {
		return domain.DevicePreference{}, err
	}
	pref.UpdatedAt = s.clock.Now()

	if err := s.repo.SaveDevicePreference(ctx, pref); err != nil {
		return domain.DevicePreference{}, fmt.Errorf("save device preference: %w", err)
	}
	return pref, nil
}

// Reorder persists an explicit drag-and-drop ordering and switches the user's
// sort to "custom" so the new order actually takes effect.
func (s *PreferenceService) Reorder(ctx context.Context, userID string, deviceIDs []string) error {
	for index, deviceID := range deviceIDs {
		if strings.TrimSpace(deviceID) == "" {
			continue
		}
		idx := index
		if _, err := s.UpsertDevicePreference(ctx, userID, deviceID, DevicePreferencePatch{SortIndex: &idx}); err != nil {
			return err
		}
	}
	custom := string(domain.SortKeyCustom)
	if _, err := s.UpdateSettings(ctx, userID, SettingsPatch{SortKey: &custom}); err != nil {
		return err
	}
	return nil
}

// DeleteDevicePreference resets one device back to its defaults.
func (s *PreferenceService) DeleteDevicePreference(ctx context.Context, userID, deviceID string) error {
	if s.icons != nil {
		if err := s.icons.DeleteForDevice(ctx, userID, deviceID); err != nil {
			return fmt.Errorf("delete icons: %w", err)
		}
	}
	return s.repo.DeleteDevicePreference(ctx, userID, deviceID)
}

// Reset removes every customisation belonging to a user.
func (s *PreferenceService) Reset(ctx context.Context, userID string) error {
	return s.repo.Reset(ctx, userID)
}

// SaveIcon validates and stores a marker image, then points the device's
// preference at it. Returns the updated preference.
func (s *PreferenceService) SaveIcon(ctx context.Context, userID, deviceID, declaredType string, data []byte) (domain.DevicePreference, error) {
	if s.icons == nil {
		return domain.DevicePreference{}, domain.NewValidationError("icon", "icon uploads are disabled")
	}
	if len(data) == 0 {
		return domain.DevicePreference{}, domain.NewValidationError("icon", "file is empty")
	}
	if int64(len(data)) > s.maxIconBytes {
		return domain.DevicePreference{}, domain.NewValidationError("icon",
			fmt.Sprintf("file is larger than %d KB", s.maxIconBytes/1024))
	}

	// Trust the bytes, not the client: sniff the real content type and require
	// that it is an allowed raster image. SVG is rejected by default because
	// serving attacker-supplied SVG from our own origin is an XSS vector.
	sniffed := normaliseContentType(http.DetectContentType(data))
	declared := normaliseContentType(declaredType)
	if !s.allowedTypes[sniffed] {
		return domain.DevicePreference{}, domain.NewValidationError("icon",
			fmt.Sprintf("unsupported image type %q", sniffed))
	}
	if declared != "" && declared != sniffed && !s.allowedTypes[declared] {
		return domain.DevicePreference{}, domain.NewValidationError("icon", "file content does not match its type")
	}

	id, err := newID()
	if err != nil {
		return domain.DevicePreference{}, err
	}

	// One icon per device: drop the previous image so uploads cannot pile up.
	if err := s.icons.DeleteForDevice(ctx, userID, deviceID); err != nil {
		return domain.DevicePreference{}, fmt.Errorf("replace icon: %w", err)
	}

	icon := domain.Icon{
		ID:          id,
		UserID:      userID,
		DeviceID:    deviceID,
		ContentType: sniffed,
		Size:        len(data),
		Data:        data,
		CreatedAt:   s.clock.Now(),
	}
	if _, err := s.icons.Save(ctx, icon); err != nil {
		return domain.DevicePreference{}, fmt.Errorf("save icon: %w", err)
	}

	pref, err := s.repo.GetDevicePreference(ctx, userID, deviceID)
	if err != nil {
		return domain.DevicePreference{}, err
	}
	pref.UserID = userID
	pref.DeviceID = deviceID
	pref.MarkerIcon = domain.IconCustom
	pref.CustomIconURL = s.iconURLPrefix + id
	pref.UpdatedAt = s.clock.Now()
	if err := pref.Validate(); err != nil {
		return domain.DevicePreference{}, err
	}
	if err := s.repo.SaveDevicePreference(ctx, pref); err != nil {
		return domain.DevicePreference{}, fmt.Errorf("save device preference: %w", err)
	}
	return pref, nil
}

// DeleteIcon removes a custom marker image and reverts to a built-in icon.
func (s *PreferenceService) DeleteIcon(ctx context.Context, userID, deviceID string) (domain.DevicePreference, error) {
	if s.icons != nil {
		if err := s.icons.DeleteForDevice(ctx, userID, deviceID); err != nil {
			return domain.DevicePreference{}, err
		}
	}
	pref, err := s.repo.GetDevicePreference(ctx, userID, deviceID)
	if err != nil {
		return domain.DevicePreference{}, err
	}
	pref.UserID = userID
	pref.DeviceID = deviceID
	pref.CustomIconURL = ""
	if pref.MarkerIcon == domain.IconCustom {
		pref.MarkerIcon = domain.IconCar
	}
	pref.UpdatedAt = s.clock.Now()
	if err := s.repo.SaveDevicePreference(ctx, pref); err != nil {
		return domain.DevicePreference{}, err
	}
	return pref, nil
}

// Icon returns a stored marker image for serving.
func (s *PreferenceService) Icon(ctx context.Context, id string) (domain.Icon, error) {
	if s.icons == nil {
		return domain.Icon{}, domain.ErrNotFound
	}
	return s.icons.Get(ctx, id)
}

// normaliseContentType strips parameters such as "; charset=utf-8".
func normaliseContentType(raw string) string {
	base, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(raw)), ";")
	return strings.TrimSpace(base)
}

// newID returns a URL-safe random identifier.
func newID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
