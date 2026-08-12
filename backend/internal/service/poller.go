package service

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"

	"fleetview/internal/domain"
)

// Event names published to realtime subscribers.
const (
	EventFleetUpdated = "fleet.updated"
	EventFleetError   = "fleet.error"
)

// PollerDeps are the collaborators of the background poller.
type PollerDeps struct {
	Provider  domain.DeviceProvider
	Cache     *SnapshotCache
	History   domain.HistoryRepository
	Publisher domain.Publisher
	Logger    *slog.Logger
	Clock     domain.Clock

	Interval         time.Duration
	Timeout          time.Duration
	FailureThreshold int

	HistoryEnabled     bool
	HistoryMinDistance float64
	HistoryRetention   time.Duration
	HistoryPrune       time.Duration
}

// lastFix remembers where a device was the last time we stored a breadcrumb.
type lastFix struct {
	lat    float64
	lng    float64
	status domain.DriveStatus
}

// Poller refreshes the fleet snapshot on a fixed interval. It is the only
// component that talks to the GPS provider, so upstream latency and outages
// are isolated from every HTTP request the dashboard makes.
type Poller struct {
	provider  domain.DeviceProvider
	cache     *SnapshotCache
	history   domain.HistoryRepository
	publisher domain.Publisher
	log       *slog.Logger
	clock     domain.Clock

	interval         time.Duration
	timeout          time.Duration
	failureThreshold int

	historyEnabled bool
	minDistance    float64
	retention      time.Duration
	pruneInterval  time.Duration

	mu                  sync.RWMutex
	consecutiveFailures int
	totalPolls          int64
	totalFailures       int64
	lastSuccess         time.Time
	lastAttempt         time.Time
	lastError           string
	lastFixes           map[string]lastFix
}

// PollerHealth is the observable state of the background loop.
type PollerHealth struct {
	Provider            string
	Healthy             bool
	ConsecutiveFailures int
	TotalPolls          int64
	TotalFailures       int64
	LastSuccess         time.Time
	LastAttempt         time.Time
	LastError           string
	IntervalSeconds     float64
}

// NewPoller wires a Poller, applying safe defaults for optional settings.
func NewPoller(deps PollerDeps) *Poller {
	if deps.Clock == nil {
		deps.Clock = domain.SystemClock{}
	}
	if deps.Interval <= 0 {
		deps.Interval = 10 * time.Second
	}
	if deps.Timeout <= 0 {
		// Generous relative to the interval: the client's own retries must be
		// allowed to finish, and a tick that arrives mid-poll is simply skipped.
		deps.Timeout = 3 * deps.Interval
	}
	if deps.FailureThreshold <= 0 {
		deps.FailureThreshold = 3
	}
	if deps.HistoryMinDistance <= 0 {
		deps.HistoryMinDistance = 15
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}

	return &Poller{
		provider:         deps.Provider,
		cache:            deps.Cache,
		history:          deps.History,
		publisher:        deps.Publisher,
		log:              deps.Logger.With("component", "poller"),
		clock:            deps.Clock,
		interval:         deps.Interval,
		timeout:          deps.Timeout,
		failureThreshold: deps.FailureThreshold,
		historyEnabled:   deps.HistoryEnabled && deps.History != nil,
		minDistance:      deps.HistoryMinDistance,
		retention:        deps.HistoryRetention,
		pruneInterval:    deps.HistoryPrune,
		lastFixes:        make(map[string]lastFix),
	}
}

// Run blocks until ctx is cancelled, polling on the configured interval. The
// first poll happens immediately so the dashboard is never empty on boot.
func (p *Poller) Run(ctx context.Context) {
	p.log.Info("starting background poller",
		"provider", p.provider.Name(),
		"interval", p.interval.String(),
		"history", p.historyEnabled)

	if p.historyEnabled && p.pruneInterval > 0 && p.retention > 0 {
		go p.pruneLoop(ctx)
	}

	_ = p.PollOnce(ctx)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.log.Info("background poller stopped")
			return
		case <-ticker.C:
			// Ticks that arrive while a slow poll is still running are
			// coalesced by the ticker, so polls never overlap.
			_ = p.PollOnce(ctx)
		}
	}
}

// PollOnce performs a single refresh cycle. It is exported so tests (and a
// future admin "refresh now" endpoint) can drive the loop deterministically.
func (p *Poller) PollOnce(ctx context.Context) error {
	attemptCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	started := p.clock.Now()
	devices, err := p.provider.FetchDevices(attemptCtx)

	p.mu.Lock()
	p.totalPolls++
	p.lastAttempt = started
	p.mu.Unlock()

	if err != nil {
		p.onFailure(err)
		return err
	}
	p.onSuccess(ctx, devices, started)
	return nil
}

func (p *Poller) onSuccess(ctx context.Context, devices []domain.Device, started time.Time) {
	snapshot := domain.Snapshot{
		Devices:   devices,
		FetchedAt: p.clock.Now(),
	}
	p.cache.Set(snapshot)

	p.mu.Lock()
	recovered := p.consecutiveFailures > 0
	p.consecutiveFailures = 0
	p.lastSuccess = snapshot.FetchedAt
	p.lastError = ""
	p.mu.Unlock()

	if recovered {
		p.log.Info("upstream data recovered", "devices", len(devices))
	}
	p.log.Debug("fleet snapshot refreshed",
		"devices", len(devices),
		"took_ms", p.clock.Now().Sub(started).Milliseconds())

	if p.historyEnabled {
		p.recordHistory(ctx, devices)
	}
	if p.publisher != nil {
		p.publisher.Publish(EventFleetUpdated, snapshot)
	}
}

func (p *Poller) onFailure(err error) {
	p.mu.Lock()
	p.consecutiveFailures++
	p.totalFailures++
	failures := p.consecutiveFailures
	p.lastError = err.Error()
	p.mu.Unlock()

	p.cache.MarkStale(err.Error())

	// The first blip is noise; sustained failure is an incident. Log
	// accordingly so alerting can key off ERROR without being flooded.
	if failures >= p.failureThreshold {
		p.log.Error("upstream fetch failing repeatedly",
			"consecutive_failures", failures, "error", err)
	} else {
		p.log.Warn("upstream fetch failed, serving cached data",
			"consecutive_failures", failures, "error", err)
	}

	if p.publisher != nil {
		p.publisher.Publish(EventFleetError, map[string]any{
			"message":             "Live GPS feed is temporarily unavailable",
			"consecutiveFailures": failures,
			"detail":              err.Error(),
		})
	}
}

// recordHistory appends breadcrumbs for devices that actually moved or changed
// state, so a parked fleet does not fill the database overnight.
func (p *Poller) recordHistory(ctx context.Context, devices []domain.Device) {
	points := make([]domain.HistoryPoint, 0, len(devices))

	p.mu.Lock()
	for _, d := range devices {
		if !d.Position.Valid() {
			continue
		}
		previous, seen := p.lastFixes[d.ID]
		moved := !seen || distanceMeters(previous.lat, previous.lng, d.Position.Lat, d.Position.Lng) >= p.minDistance
		changed := seen && previous.status != d.DriveStatus
		if seen && !moved && !changed {
			continue
		}

		p.lastFixes[d.ID] = lastFix{lat: d.Position.Lat, lng: d.Position.Lng, status: d.DriveStatus}
		points = append(points, domain.HistoryPoint{
			DeviceID:    d.ID,
			Lat:         d.Position.Lat,
			Lng:         d.Position.Lng,
			Speed:       d.Position.Speed,
			Heading:     d.Position.Heading,
			DriveStatus: d.DriveStatus,
			RecordedAt:  d.Position.RecordedAt,
		})
	}
	p.mu.Unlock()

	if len(points) == 0 {
		return
	}
	if err := p.history.Append(ctx, points); err != nil {
		p.log.Error("failed to persist device history", "points", len(points), "error", err)
	}
}

// pruneLoop deletes breadcrumbs older than the retention window.
func (p *Poller) pruneLoop(ctx context.Context) {
	ticker := time.NewTicker(p.pruneInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := p.clock.Now().Add(-p.retention)
			removed, err := p.history.Prune(ctx, cutoff)
			if err != nil {
				p.log.Error("history prune failed", "error", err)
				continue
			}
			if removed > 0 {
				p.log.Info("pruned device history", "rows", removed, "older_than", cutoff.UTC())
			}
		}
	}
}

// Health reports the loop's current state for /healthz and the UI banner.
func (p *Poller) Health() PollerHealth {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return PollerHealth{
		Provider:            p.provider.Name(),
		Healthy:             p.consecutiveFailures < p.failureThreshold,
		ConsecutiveFailures: p.consecutiveFailures,
		TotalPolls:          p.totalPolls,
		TotalFailures:       p.totalFailures,
		LastSuccess:         p.lastSuccess,
		LastAttempt:         p.lastAttempt,
		LastError:           p.lastError,
		IntervalSeconds:     p.interval.Seconds(),
	}
}

// distanceMeters is the great-circle distance between two coordinates.
func distanceMeters(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusM = 6371000.0
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusM * math.Asin(math.Min(1, math.Sqrt(a)))
}
