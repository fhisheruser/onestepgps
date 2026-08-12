package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fleetview/internal/domain"
)

var baseTime = time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)

func sampleDevices() []domain.Device {
	return []domain.Device{
		{
			ID: "d1", Name: "Truck 04", Make: "Freightliner", Model: "M2 106",
			Online: true, Active: true, DriveStatus: domain.DriveStatusDriving,
			Groups:   []string{"Logistics"},
			Position: domain.Position{Lat: 32.7, Lng: -117.1, Speed: 55, RecordedAt: baseTime},
		},
		{
			ID: "d2", Name: "Van 12", Make: "Ford", Model: "Transit",
			Online: true, Active: true, DriveStatus: domain.DriveStatusIdle,
			Groups:   []string{"Service"},
			Position: domain.Position{Lat: 32.8, Lng: -117.2, Speed: 0, RecordedAt: baseTime.Add(-time.Minute)},
		},
		{
			ID: "d3", Name: "Bus 02", Make: "Blue Bird", Model: "Vision",
			Online: false, Active: true, DriveStatus: domain.DriveStatusOff,
			Groups:   []string{"Transit"},
			Position: domain.Position{Lat: 32.9, Lng: -117.3, Speed: 0, RecordedAt: baseTime.Add(-2 * time.Hour)},
		},
	}
}

func TestMerge_AppliesPreferencesAndDefaults(t *testing.T) {
	prefs := map[string]domain.DevicePreference{
		"d1": {
			DeviceID: "d1", DisplayName: "Harbor Hauler", Pinned: true,
			MarkerColor: "#2E7D32", MarkerIcon: domain.IconTruck, Notes: "Refrigerated",
		},
		"d2": {DeviceID: "d2", Hidden: true},
	}

	views := Merge(sampleDevices(), prefs)
	require.Len(t, views, 3)

	assert.Equal(t, "Harbor Hauler", views[0].DisplayName)
	assert.True(t, views[0].Renamed)
	assert.True(t, views[0].Pinned)
	assert.Equal(t, "#2E7D32", views[0].MarkerColor)
	assert.Equal(t, "Refrigerated", views[0].Notes)
	// The provider's own name is preserved so the UI can show "renamed from".
	assert.Equal(t, "Truck 04", views[0].Device.Name)

	assert.True(t, views[1].Hidden)
	assert.False(t, views[1].Renamed)
	assert.Equal(t, "Van 12", views[1].DisplayName)

	// Untouched devices get defaults, with the icon inferred from the vehicle.
	assert.Equal(t, domain.DefaultMarkerColor, views[2].MarkerColor)
	assert.Equal(t, domain.IconBus, views[2].MarkerIcon)
}

func TestMerge_InfersIconFromVehicleDescription(t *testing.T) {
	cases := map[string]domain.MarkerIcon{
		"Truck 04":        domain.IconTruck,
		"Van 12":          domain.IconVan,
		"Bus 02":          domain.IconBus,
		"Pickup 07":       domain.IconPickup,
		"Sales Car Alpha": domain.IconCar,
	}
	for name, expected := range cases {
		t.Run(name, func(t *testing.T) {
			views := Merge([]domain.Device{{ID: "x", Name: name}}, nil)
			assert.Equal(t, expected, views[0].MarkerIcon)
		})
	}
}

func TestMerge_FallsBackWhenCustomIconIsMissing(t *testing.T) {
	prefs := map[string]domain.DevicePreference{
		"d1": {DeviceID: "d1", MarkerIcon: domain.IconCustom, CustomIconURL: ""},
	}
	views := Merge(sampleDevices()[:1], prefs)
	assert.Equal(t, domain.IconTruck, views[0].MarkerIcon, "a dangling custom icon must not render as broken")
}

func TestFilter_HidesDevicesUnlessRequested(t *testing.T) {
	views := Merge(sampleDevices(), map[string]domain.DevicePreference{
		"d2": {DeviceID: "d2", Hidden: true},
	})

	visible := Filter(views, domain.DeviceQuery{Status: domain.StatusAll})
	require.Len(t, visible, 2)
	for _, v := range visible {
		assert.NotEqual(t, "d2", v.ID)
	}

	withHidden := Filter(views, domain.DeviceQuery{Status: domain.StatusAll, IncludeHidden: true})
	assert.Len(t, withHidden, 3)
}

func TestFilter_ByStatus(t *testing.T) {
	views := Merge(sampleDevices(), nil)

	assert.Len(t, Filter(views, domain.DeviceQuery{Status: domain.StatusDriving}), 1)
	assert.Len(t, Filter(views, domain.DeviceQuery{Status: domain.StatusIdle}), 1)
	assert.Len(t, Filter(views, domain.DeviceQuery{Status: domain.StatusOffline}), 1)
	assert.Len(t, Filter(views, domain.DeviceQuery{Status: domain.StatusOnline}), 2)
	assert.Len(t, Filter(views, domain.DeviceQuery{Status: domain.StatusAll}), 3)
}

func TestFilter_SearchesEveryUsefulField(t *testing.T) {
	views := Merge(sampleDevices(), map[string]domain.DevicePreference{
		"d1": {DeviceID: "d1", DisplayName: "Harbor Hauler", Notes: "refrigerated"},
	})

	for _, needle := range []string{"harbor", "HARBOR", "Truck 04", "freightliner", "logistics", "d1", "refrigerated"} {
		t.Run(needle, func(t *testing.T) {
			got := Filter(views, domain.DeviceQuery{Status: domain.StatusAll, Search: needle})
			require.Len(t, got, 1)
			assert.Equal(t, "d1", got[0].ID)
		})
	}

	assert.Empty(t, Filter(views, domain.DeviceQuery{Status: domain.StatusAll, Search: "helicopter"}))
}

func TestSort_ByName(t *testing.T) {
	views := Merge(sampleDevices(), nil)

	Sort(views, domain.SortKeyName, domain.SortAsc)
	assert.Equal(t, []string{"Bus 02", "Truck 04", "Van 12"}, names(views))

	Sort(views, domain.SortKeyName, domain.SortDesc)
	assert.Equal(t, []string{"Van 12", "Truck 04", "Bus 02"}, names(views))
}

func TestSort_PinnedAlwaysFirst(t *testing.T) {
	views := Merge(sampleDevices(), map[string]domain.DevicePreference{
		"d2": {DeviceID: "d2", Pinned: true},
	})

	Sort(views, domain.SortKeyName, domain.SortAsc)
	assert.Equal(t, "Van 12", views[0].DisplayName, "pinned wins over the sort key")

	Sort(views, domain.SortKeyName, domain.SortDesc)
	assert.Equal(t, "Van 12", views[0].DisplayName, "…in both directions")
}

func TestSort_BySpeedStatusAndRecency(t *testing.T) {
	views := Merge(sampleDevices(), nil)

	Sort(views, domain.SortKeySpeed, domain.SortAsc)
	assert.Equal(t, "Truck 04", views[0].DisplayName, "fastest first")

	Sort(views, domain.SortKeyStatus, domain.SortAsc)
	assert.Equal(t, domain.DriveStatusDriving, views[0].DriveStatus)
	assert.Equal(t, domain.DriveStatusOff, views[2].DriveStatus)

	Sort(views, domain.SortKeyUpdated, domain.SortAsc)
	assert.Equal(t, "Truck 04", views[0].DisplayName, "most recent first")
	assert.Equal(t, "Bus 02", views[2].DisplayName)
}

func TestSort_CustomOrderThenStableTieBreak(t *testing.T) {
	views := Merge(sampleDevices(), map[string]domain.DevicePreference{
		"d3": {DeviceID: "d3", SortIndex: 1},
		"d1": {DeviceID: "d1", SortIndex: 2},
		"d2": {DeviceID: "d2", SortIndex: 3},
	})

	Sort(views, domain.SortKeyCustom, domain.SortAsc)
	assert.Equal(t, []string{"Bus 02", "Truck 04", "Van 12"}, names(views))

	// Equal keys fall back to name so rows do not jitter between polls.
	equal := Merge(sampleDevices(), nil)
	Sort(equal, domain.SortKeyCustom, domain.SortAsc)
	assert.Equal(t, []string{"Bus 02", "Truck 04", "Van 12"}, names(equal))
}

func TestSummarize_CountsFleetState(t *testing.T) {
	views := Merge(sampleDevices(), map[string]domain.DevicePreference{
		"d3": {DeviceID: "d3", Hidden: true},
	})
	candidates := Filter(views, domain.DeviceQuery{Status: domain.StatusAll})

	snapshot := domain.Snapshot{FetchedAt: baseTime, Devices: sampleDevices()}
	summary := Summarize(views, candidates, snapshot, "mph")

	assert.Equal(t, 3, summary.Total)
	assert.Equal(t, 2, summary.Visible)
	assert.Equal(t, 1, summary.Hidden)
	assert.Equal(t, 2, summary.Online)
	assert.Equal(t, 0, summary.Offline, "hidden devices are excluded from the KPIs")
	assert.Equal(t, 1, summary.Driving)
	assert.Equal(t, 1, summary.Idle)
	assert.InDelta(t, 55, summary.AvgSpeed, 0.01, "average speed covers moving vehicles only")
	assert.Equal(t, "mph", summary.SpeedUnit)
	assert.Equal(t, baseTime, summary.LastUpdated)
}

func TestPreferenceIndex(t *testing.T) {
	index := PreferenceIndex([]domain.DevicePreference{
		{DeviceID: "a", Pinned: true},
		{DeviceID: "b"},
	})
	require.Len(t, index, 2)
	assert.True(t, index["a"].Pinned)
}

func names(views []domain.DeviceView) []string {
	out := make([]string, 0, len(views))
	for _, v := range views {
		out = append(out, v.DisplayName)
	}
	return out
}
