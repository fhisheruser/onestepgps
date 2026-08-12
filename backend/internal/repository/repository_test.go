package repository

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"fleetview/internal/domain"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := Open(Options{
		Path:        filepath.Join(t.TempDir(), "test.db"),
		AutoMigrate: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = Close(db) })
	return db
}

func TestGetSettings_ReturnsDefaultsForUnknownUser(t *testing.T) {
	repo := NewPreferenceRepository(newTestDB(t))

	settings, err := repo.GetSettings(context.Background(), "alice")
	require.NoError(t, err)

	assert.Equal(t, "alice", settings.UserID)
	assert.Equal(t, domain.SortKeyName, settings.SortKey)
	assert.Equal(t, domain.SpeedMPH, settings.SpeedUnit)
	assert.Equal(t, 10, settings.RefreshSeconds)
	assert.True(t, settings.ShowOfflineDevices)
}

func TestSaveSettings_RoundTrips(t *testing.T) {
	repo := NewPreferenceRepository(newTestDB(t))
	ctx := context.Background()

	settings := domain.DefaultUserSettings("alice")
	settings.Theme = domain.ThemeDark
	settings.SortKey = domain.SortKeySpeed
	settings.SortDirection = domain.SortDesc
	settings.SpeedUnit = domain.SpeedKPH
	settings.MapType = "hybrid"
	settings.ShowOfflineDevices = false
	settings.ShowTrails = true
	settings.RefreshSeconds = 30

	require.NoError(t, repo.SaveSettings(ctx, settings))

	loaded, err := repo.GetSettings(ctx, "alice")
	require.NoError(t, err)
	assert.Equal(t, domain.ThemeDark, loaded.Theme)
	assert.Equal(t, domain.SortKeySpeed, loaded.SortKey)
	assert.Equal(t, domain.SortDesc, loaded.SortDirection)
	assert.Equal(t, domain.SpeedKPH, loaded.SpeedUnit)
	assert.Equal(t, "hybrid", loaded.MapType)
	assert.False(t, loaded.ShowOfflineDevices, "false must survive the round trip")
	assert.True(t, loaded.ShowTrails)
	assert.Equal(t, 30, loaded.RefreshSeconds)


	settings.Theme = domain.ThemeLight
	require.NoError(t, repo.SaveSettings(ctx, settings))
	loaded, err = repo.GetSettings(ctx, "alice")
	require.NoError(t, err)
	assert.Equal(t, domain.ThemeLight, loaded.Theme)
}

func TestSaveSettings_NormalisesOutOfRangeValues(t *testing.T) {
	repo := NewPreferenceRepository(newTestDB(t))
	ctx := context.Background()

	settings := domain.DefaultUserSettings("alice")
	settings.RefreshSeconds = 99999
	settings.SortKey = "nonsense"
	settings.MapType = "flat-earth"
	require.NoError(t, repo.SaveSettings(ctx, settings))

	loaded, err := repo.GetSettings(ctx, "alice")
	require.NoError(t, err)
	assert.Equal(t, domain.MaxRefreshSeconds, loaded.RefreshSeconds)
	assert.Equal(t, domain.SortKeyName, loaded.SortKey)
	assert.Equal(t, "roadmap", loaded.MapType)
}

func TestSaveSettings_RequiresUserID(t *testing.T) {
	repo := NewPreferenceRepository(newTestDB(t))
	err := repo.SaveSettings(context.Background(), domain.UserSettings{})
	require.Error(t, err)
	_, ok := domain.AsValidationError(err)
	assert.True(t, ok)
}

func TestDevicePreferences_UpsertListAndDelete(t *testing.T) {
	repo := NewPreferenceRepository(newTestDB(t))
	ctx := context.Background()

	pref := domain.DevicePreference{
		UserID: "alice", DeviceID: "d1",
		DisplayName: "Harbor Hauler", MarkerColor: "#2E7D32",
		MarkerIcon: domain.IconTruck, Pinned: true, Notes: "Refrigerated",
	}
	require.NoError(t, repo.SaveDevicePreference(ctx, pref))

	loaded, err := repo.GetDevicePreference(ctx, "alice", "d1")
	require.NoError(t, err)
	assert.Equal(t, "Harbor Hauler", loaded.DisplayName)
	assert.Equal(t, domain.IconTruck, loaded.MarkerIcon)
	assert.True(t, loaded.Pinned)


	pref.DisplayName = "Harbor Hauler II"
	require.NoError(t, repo.SaveDevicePreference(ctx, pref))

	list, err := repo.ListDevicePreferences(ctx, "alice")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Harbor Hauler II", list[0].DisplayName)

	require.NoError(t, repo.DeleteDevicePreference(ctx, "alice", "d1"))
	list, err = repo.ListDevicePreferences(ctx, "alice")
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestGetDevicePreference_UnknownDeviceIsNotAnError(t *testing.T) {
	repo := NewPreferenceRepository(newTestDB(t))

	pref, err := repo.GetDevicePreference(context.Background(), "alice", "never-seen")
	require.NoError(t, err, "callers patch the result, so missing must not fail")
	assert.Equal(t, "alice", pref.UserID)
	assert.Equal(t, "never-seen", pref.DeviceID)
	assert.True(t, pref.IsZeroValue())
}

func TestSaveDevicePreference_DropsRowWithNoCustomisation(t *testing.T) {
	repo := NewPreferenceRepository(newTestDB(t))
	ctx := context.Background()

	require.NoError(t, repo.SaveDevicePreference(ctx, domain.DevicePreference{
		UserID: "alice", DeviceID: "d1", Hidden: true,
	}))
	list, err := repo.ListDevicePreferences(ctx, "alice")
	require.NoError(t, err)
	require.Len(t, list, 1)

	
	require.NoError(t, repo.SaveDevicePreference(ctx, domain.DevicePreference{
		UserID: "alice", DeviceID: "d1", Hidden: false,
	}))
	list, err = repo.ListDevicePreferences(ctx, "alice")
	require.NoError(t, err)
	assert.Empty(t, list, "empty preferences are deleted instead of stored")
}

func TestPreferences_AreScopedPerUser(t *testing.T) {
	repo := NewPreferenceRepository(newTestDB(t))
	ctx := context.Background()

	require.NoError(t, repo.SaveDevicePreference(ctx, domain.DevicePreference{UserID: "alice", DeviceID: "d1", Hidden: true}))
	require.NoError(t, repo.SaveDevicePreference(ctx, domain.DevicePreference{UserID: "bob", DeviceID: "d1", Pinned: true}))

	alice, err := repo.GetDevicePreference(ctx, "alice", "d1")
	require.NoError(t, err)
	bob, err := repo.GetDevicePreference(ctx, "bob", "d1")
	require.NoError(t, err)

	assert.True(t, alice.Hidden)
	assert.False(t, alice.Pinned)
	assert.True(t, bob.Pinned)
	assert.False(t, bob.Hidden)
}

func TestReset_RemovesEverythingForOneUserOnly(t *testing.T) {
	db := newTestDB(t)
	prefs := NewPreferenceRepository(db)
	icons := NewIconRepository(db)
	ctx := context.Background()

	require.NoError(t, prefs.SaveSettings(ctx, domain.DefaultUserSettings("alice")))
	require.NoError(t, prefs.SaveDevicePreference(ctx, domain.DevicePreference{UserID: "alice", DeviceID: "d1", Pinned: true}))
	_, err := icons.Save(ctx, domain.Icon{ID: "icon1", UserID: "alice", DeviceID: "d1", ContentType: "image/png", Size: 3, Data: []byte{1, 2, 3}})
	require.NoError(t, err)
	require.NoError(t, prefs.SaveDevicePreference(ctx, domain.DevicePreference{UserID: "bob", DeviceID: "d1", Pinned: true}))

	require.NoError(t, prefs.Reset(ctx, "alice"))

	list, err := prefs.ListDevicePreferences(ctx, "alice")
	require.NoError(t, err)
	assert.Empty(t, list)

	_, err = icons.Get(ctx, "icon1")
	assert.True(t, errors.Is(err, domain.ErrNotFound))

	bobList, err := prefs.ListDevicePreferences(ctx, "bob")
	require.NoError(t, err)
	assert.Len(t, bobList, 1, "one user's reset must not touch another's data")
}

func TestIconRepository_SaveGetReplace(t *testing.T) {
	repo := NewIconRepository(newTestDB(t))
	ctx := context.Background()

	saved, err := repo.Save(ctx, domain.Icon{
		ID: "icon1", UserID: "alice", DeviceID: "d1",
		ContentType: "image/png", Size: 4, Data: []byte{0x89, 0x50, 0x4e, 0x47},
	})
	require.NoError(t, err)
	assert.Equal(t, "icon1", saved.ID)

	loaded, err := repo.Get(ctx, "icon1")
	require.NoError(t, err)
	assert.Equal(t, "image/png", loaded.ContentType)
	assert.Equal(t, []byte{0x89, 0x50, 0x4e, 0x47}, loaded.Data)

	require.NoError(t, repo.DeleteForDevice(ctx, "alice", "d1"))
	_, err = repo.Get(ctx, "icon1")
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}

func TestHistoryRepository_AppendListPrune(t *testing.T) {
	repo := NewHistoryRepository(newTestDB(t))
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	points := []domain.HistoryPoint{
		{DeviceID: "d1", Lat: 32.70, Lng: -117.1, DriveStatus: domain.DriveStatusDriving, RecordedAt: now.Add(-30 * time.Minute)},
		{DeviceID: "d1", Lat: 32.71, Lng: -117.1, DriveStatus: domain.DriveStatusDriving, RecordedAt: now.Add(-20 * time.Minute)},
		{DeviceID: "d1", Lat: 32.72, Lng: -117.1, DriveStatus: domain.DriveStatusIdle, RecordedAt: now.Add(-10 * time.Minute)},
		{DeviceID: "d2", Lat: 33.00, Lng: -117.2, DriveStatus: domain.DriveStatusOff, RecordedAt: now.Add(-5 * time.Minute)},
		{DeviceID: "", Lat: 1, Lng: 1, RecordedAt: now},
	}
	require.NoError(t, repo.Append(ctx, points))
	require.NoError(t, repo.Append(ctx, nil), "an empty batch is a no-op")

	trail, err := repo.List(ctx, "d1", now.Add(-time.Hour), 100)
	require.NoError(t, err)
	require.Len(t, trail, 3)
	assert.True(t, trail[0].RecordedAt.Before(trail[2].RecordedAt), "trails come back oldest first")
	assert.InDelta(t, 32.70, trail[0].Lat, 0.0001)

	recent, err := repo.List(ctx, "d1", now.Add(-15*time.Minute), 100)
	require.NoError(t, err)
	assert.Len(t, recent, 1)

	
	limited, err := repo.List(ctx, "d1", now.Add(-time.Hour), 2)
	require.NoError(t, err)
	require.Len(t, limited, 2)
	assert.InDelta(t, 32.72, limited[1].Lat, 0.0001)

	removed, err := repo.Prune(ctx, now.Add(-15*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, int64(2), removed)

	left, err := repo.List(ctx, "d1", now.Add(-time.Hour), 100)
	require.NoError(t, err)
	assert.Len(t, left, 1)
}
