package service

import (
	"sort"
	"strings"
	"time"

	"fleetview/internal/domain"
)


func Merge(devices []domain.Device, prefs map[string]domain.DevicePreference) []domain.DeviceView {
	views := make([]domain.DeviceView, 0, len(devices))
	for _, device := range devices {
		view := domain.DeviceView{
			Device:      device,
			DisplayName: device.Name,
			MarkerIcon:  inferIcon(device),
			MarkerColor: domain.DefaultMarkerColor,
		}

		if pref, ok := prefs[device.ID]; ok {
			if pref.DisplayName != "" {
				view.DisplayName = pref.DisplayName
				view.Renamed = true
			}
			view.Hidden = pref.Hidden
			view.Pinned = pref.Pinned
			view.Notes = pref.Notes
			view.SortIndex = pref.SortIndex
			view.CustomIconURL = pref.CustomIconURL
			if pref.MarkerIcon != "" {
				view.MarkerIcon = pref.MarkerIcon
			}
			if pref.MarkerColor != "" {
				view.MarkerColor = pref.MarkerColor
			}
		}

		
		if view.MarkerIcon == domain.IconCustom && view.CustomIconURL == "" {
			view.MarkerIcon = inferIcon(device)
		}

		views = append(views, view)
	}
	return views
}


func inferIcon(d domain.Device) domain.MarkerIcon {
	haystack := strings.ToLower(strings.Join([]string{d.Name, d.Make, d.Model}, " "))
	switch {
	case containsAny(haystack, "bus", "coach", "shuttle"):
		return domain.IconBus
	case containsAny(haystack, "truck", "freight", "peterbilt", "kenworth", "semi", "hauler", "npr"):
		return domain.IconTruck
	case containsAny(haystack, "van", "transit", "sprinter", "promaster"):
		return domain.IconVan
	case containsAny(haystack, "pickup", "silverado", "sierra", "f-150", "f150", "ram 1500", "tacoma"):
		return domain.IconPickup
	default:
		return domain.IconCar
	}
}

func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}


func Filter(views []domain.DeviceView, q domain.DeviceQuery) []domain.DeviceView {
	needle := strings.ToLower(strings.TrimSpace(q.Search))
	out := make([]domain.DeviceView, 0, len(views))

	for _, v := range views {
		if v.Hidden && !q.IncludeHidden {
			continue
		}
		if q.OnlyPinned && !v.Pinned {
			continue
		}
		if !q.Matches(v) {
			continue
		}
		if needle != "" && !matchesSearch(v, needle) {
			continue
		}
		out = append(out, v)
	}
	return out
}


func matchesSearch(v domain.DeviceView, needle string) bool {
	fields := []string{v.DisplayName, v.Name, v.Make, v.Model, v.ID, v.FactoryID, v.Notes}
	for _, f := range fields {
		if f != "" && strings.Contains(strings.ToLower(f), needle) {
			return true
		}
	}
	for _, g := range v.Groups {
		if strings.Contains(strings.ToLower(g), needle) {
			return true
		}
	}
	return false
}


var driveStatusRank = map[domain.DriveStatus]int{
	domain.DriveStatusDriving: 0,
	domain.DriveStatusIdle:    1,
	domain.DriveStatusOff:     2,
	domain.DriveStatusUnknown: 3,
}


func Sort(views []domain.DeviceView, key domain.SortKey, dir domain.SortDirection) {
	descending := dir == domain.SortDesc

	sort.SliceStable(views, func(i, j int) bool {
		a, b := views[i], views[j]
		if a.Pinned != b.Pinned {
			return a.Pinned
		}

		less, equal := compareBy(a, b, key)
		if equal {
			
			return strings.ToLower(a.DisplayName) < strings.ToLower(b.DisplayName)
		}
		if descending {
			return !less
		}
		return less
	})
}


func compareBy(a, b domain.DeviceView, key domain.SortKey) (less, equal bool) {
	switch key {
	case domain.SortKeyStatus:
		ra, rb := driveStatusRank[a.DriveStatus], driveStatusRank[b.DriveStatus]
		if ra == rb {
			return false, true
		}
		return ra < rb, false

	case domain.SortKeySpeed:
		if a.Position.Speed == b.Position.Speed {
			return false, true
		}
	
		return a.Position.Speed > b.Position.Speed, false

	case domain.SortKeyUpdated:
		at, bt := a.Position.RecordedAt, b.Position.RecordedAt
		if at.Equal(bt) {
			return false, true
		}
		return at.After(bt), false

	case domain.SortKeyCustom:
		if a.SortIndex == b.SortIndex {
			return false, true
		}
		return a.SortIndex < b.SortIndex, false

	default: 
		an, bn := strings.ToLower(a.DisplayName), strings.ToLower(b.DisplayName)
		if an == bn {
			return false, true
		}
		return an < bn, false
	}
}


func Summarize(all, candidates []domain.DeviceView, snap domain.Snapshot, unit string) domain.FleetSummary {
	summary := domain.FleetSummary{
		Total:       len(all),
		Visible:     len(candidates),
		SpeedUnit:   unit,
		LastUpdated: snap.FetchedAt,
		Stale:       snap.Stale,
	}

	for _, v := range all {
		if v.Hidden {
			summary.Hidden++
		}
	}

	var speedSum float64
	var moving int
	for _, v := range candidates {
		if v.Online {
			summary.Online++
		} else {
			summary.Offline++
		}
		switch v.DriveStatus {
		case domain.DriveStatusDriving:
			summary.Driving++
			speedSum += v.Position.Speed
			moving++
		case domain.DriveStatusIdle:
			summary.Idle++
		case domain.DriveStatusOff:
			summary.Off++
		}
	}
	if moving > 0 {
		summary.AvgSpeed = speedSum / float64(moving)
	}
	return summary
}


func PreferenceIndex(prefs []domain.DevicePreference) map[string]domain.DevicePreference {
	index := make(map[string]domain.DevicePreference, len(prefs))
	for _, p := range prefs {
		index[p.DeviceID] = p
	}
	return index
}


func StalenessOf(snap domain.Snapshot, now time.Time) time.Duration {
	return snap.Age(now)
}
