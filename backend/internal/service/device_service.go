package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"fleetview/internal/domain"
)


type Feed struct {
	Devices  []domain.DeviceView
	Summary  domain.FleetSummary
	Settings domain.UserSettings
	Snapshot domain.Snapshot
	Query    domain.DeviceQuery
	Age      time.Duration
}


type DeviceServiceDeps struct {
	Cache            domain.SnapshotStore
	Preferences      domain.PreferenceRepository
	History          domain.HistoryRepository
	Clock            domain.Clock
	MaxHistoryPoints int
}


type DeviceService struct {
	cache            domain.SnapshotStore
	prefs            domain.PreferenceRepository
	history          domain.HistoryRepository
	clock            domain.Clock
	maxHistoryPoints int
}


func NewDeviceService(deps DeviceServiceDeps) *DeviceService {
	if deps.Clock == nil {
		deps.Clock = domain.SystemClock{}
	}
	if deps.MaxHistoryPoints <= 0 {
		deps.MaxHistoryPoints = 500
	}
	return &DeviceService{
		cache:            deps.Cache,
		prefs:            deps.Preferences,
		history:          deps.History,
		clock:            deps.Clock,
		maxHistoryPoints: deps.MaxHistoryPoints,
	}
}


func (s *DeviceService) Feed(ctx context.Context, userID string, q domain.DeviceQuery) (Feed, error) {
	settings, err := s.prefs.GetSettings(ctx, userID)
	if err != nil {
		return Feed{}, fmt.Errorf("load settings: %w", err)
	}

	devicePrefs, err := s.prefs.ListDevicePreferences(ctx, userID)
	if err != nil {
		return Feed{}, fmt.Errorf("load device preferences: %w", err)
	}

	q = resolveQuery(q, settings)
	snapshot := s.cache.Get()

	all := Merge(snapshot.Devices, PreferenceIndex(devicePrefs))

	
	candidates := Filter(all, domain.DeviceQuery{Status: domain.StatusAll, IncludeHidden: q.IncludeHidden})
	visible := Filter(all, q)
	Sort(visible, q.SortKey, q.SortDirection)

	return Feed{
		Devices:  visible,
		Summary:  Summarize(all, candidates, snapshot, string(settings.SpeedUnit)),
		Settings: settings,
		Snapshot: snapshot,
		Query:    q,
		Age:      snapshot.Age(s.clock.Now()),
	}, nil
}


func resolveQuery(q domain.DeviceQuery, settings domain.UserSettings) domain.DeviceQuery {
	if q.SortKey == "" {
		q.SortKey = settings.SortKey
	}
	if q.SortDirection == "" {
		q.SortDirection = settings.SortDirection
	}
	
	if q.Status == "" {
		q.Status = domain.StatusAll
		if !settings.ShowOfflineDevices {
			q.Status = domain.StatusOnline
		}
	}
	return q
}


func (s *DeviceService) Device(ctx context.Context, userID, deviceID string) (domain.DeviceView, error) {
	feed, err := s.Feed(ctx, userID, domain.DeviceQuery{IncludeHidden: true, Status: domain.StatusAll})
	if err != nil {
		return domain.DeviceView{}, err
	}
	for _, d := range feed.Devices {
		if d.ID == deviceID {
			return d, nil
		}
	}
	return domain.DeviceView{}, fmt.Errorf("device %q: %w", deviceID, domain.ErrNotFound)
}


func (s *DeviceService) History(ctx context.Context, deviceID string, window time.Duration, limit int) ([]domain.HistoryPoint, error) {
	if s.history == nil {
		return nil, nil
	}
	if window <= 0 {
		window = time.Hour
	}
	if limit <= 0 || limit > s.maxHistoryPoints {
		limit = s.maxHistoryPoints
	}
	since := s.clock.Now().Add(-window)
	return s.history.List(ctx, deviceID, since, limit)
}


func (s *DeviceService) Snapshot() domain.Snapshot { return s.cache.Get() }


func (s *DeviceService) ExportCSV(ctx context.Context, userID string, q domain.DeviceQuery) ([]byte, error) {
	feed, err := s.Feed(ctx, userID, q)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	header := []string{
		"device_id", "display_name", "provider_name", "make", "model",
		"drive_status", "online", "active", "speed", "speed_unit",
		"latitude", "longitude", "heading", "altitude_m",
		"odometer", "odometer_unit", "last_update_utc", "status_duration",
		"groups", "pinned", "hidden", "notes",
	}
	if err := w.Write(header); err != nil {
		return nil, fmt.Errorf("write csv header: %w", err)
	}

	for _, d := range feed.Devices {
		record := []string{
			d.ID,
			d.DisplayName,
			d.Name,
			d.Make,
			d.Model,
			string(d.DriveStatus),
			strconv.FormatBool(d.Online),
			strconv.FormatBool(d.Active),
			strconv.FormatFloat(d.Position.Speed, 'f', 2, 64),
			d.Position.SpeedUnit,
			strconv.FormatFloat(d.Position.Lat, 'f', 6, 64),
			strconv.FormatFloat(d.Position.Lng, 'f', 6, 64),
			strconv.FormatFloat(d.Position.Heading, 'f', 1, 64),
			strconv.FormatFloat(d.Position.Altitude, 'f', 1, 64),
			strconv.FormatFloat(d.Odometer, 'f', 1, 64),
			d.OdometerUnit,
			formatTime(d.Position.RecordedAt),
			d.DriveStatusDuration.String(),
			joinGroups(d.Groups),
			strconv.FormatBool(d.Pinned),
			strconv.FormatBool(d.Hidden),
			d.Notes,
		}
		if err := w.Write(record); err != nil {
			return nil, fmt.Errorf("write csv row: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("flush csv: %w", err)
	}
	return buf.Bytes(), nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func joinGroups(groups []string) string {
	out := ""
	for i, g := range groups {
		if i > 0 {
			out += "; "
		}
		out += g
	}
	return out
}
