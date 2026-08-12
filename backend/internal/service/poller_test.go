package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fleetview/internal/domain"
)



type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock(t time.Time) *fakeClock { return &fakeClock{now: t} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type fakeProvider struct {
	mu      sync.Mutex
	devices []domain.Device
	err     error
	calls   int
}

func (p *fakeProvider) Name() string { return "fake" }

func (p *fakeProvider) FetchDevices(context.Context) ([]domain.Device, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	return p.devices, nil
}

func (p *fakeProvider) set(devices []domain.Device, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.devices, p.err = devices, err
}

func (p *fakeProvider) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

type fakeHistory struct {
	mu     sync.Mutex
	points []domain.HistoryPoint
	pruned time.Time
}

func (h *fakeHistory) Append(_ context.Context, points []domain.HistoryPoint) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.points = append(h.points, points...)
	return nil
}

func (h *fakeHistory) List(context.Context, string, time.Time, int) ([]domain.HistoryPoint, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.points, nil
}

func (h *fakeHistory) Prune(_ context.Context, before time.Time) (int64, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pruned = before
	return 0, nil
}

func (h *fakeHistory) all() []domain.HistoryPoint {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]domain.HistoryPoint(nil), h.points...)
}

type fakePublisher struct {
	mu     sync.Mutex
	events []string
}

func (p *fakePublisher) Publish(event string, _ any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, event)
}

func (p *fakePublisher) ClientCount() int { return 1 }

func (p *fakePublisher) all() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.events...)
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func deviceAt(id string, lat, lng float64, status domain.DriveStatus, at time.Time) domain.Device {
	return domain.Device{
		ID:          id,
		Name:        id,
		Online:      true,
		Active:      true,
		DriveStatus: status,
		Position:    domain.Position{Lat: lat, Lng: lng, RecordedAt: at, SpeedUnit: "km/h"},
	}
}



func TestPollOnce_PublishesSnapshot(t *testing.T) {
	clock := newFakeClock(baseTime)
	provider := &fakeProvider{devices: []domain.Device{deviceAt("d1", 32.7, -117.1, domain.DriveStatusDriving, baseTime)}}
	cache := NewSnapshotCache()
	publisher := &fakePublisher{}

	poller := NewPoller(PollerDeps{
		Provider: provider, Cache: cache, Publisher: publisher,
		Logger: quietLogger(), Clock: clock, Interval: time.Second,
	})

	require.NoError(t, poller.PollOnce(context.Background()))

	snapshot := cache.Get()
	require.Len(t, snapshot.Devices, 1)
	assert.False(t, snapshot.Stale)
	assert.Equal(t, baseTime, snapshot.FetchedAt)
	assert.Equal(t, []string{EventFleetUpdated}, publisher.all())

	health := poller.Health()
	assert.True(t, health.Healthy)
	assert.Equal(t, int64(1), health.TotalPolls)
	assert.Zero(t, health.ConsecutiveFailures)
}

func TestPollOnce_KeepsLastGoodDataOnFailure(t *testing.T) {
	clock := newFakeClock(baseTime)
	provider := &fakeProvider{devices: []domain.Device{deviceAt("d1", 32.7, -117.1, domain.DriveStatusDriving, baseTime)}}
	cache := NewSnapshotCache()
	publisher := &fakePublisher{}

	poller := NewPoller(PollerDeps{
		Provider: provider, Cache: cache, Publisher: publisher,
		Logger: quietLogger(), Clock: clock, Interval: time.Second, FailureThreshold: 2,
	})
	require.NoError(t, poller.PollOnce(context.Background()))

	provider.set(nil, errors.New("upstream exploded"))
	require.Error(t, poller.PollOnce(context.Background()))

	snapshot := cache.Get()
	require.Len(t, snapshot.Devices, 1, "the dashboard keeps showing the last known fleet")
	assert.True(t, snapshot.Stale)
	assert.Contains(t, snapshot.Error, "upstream exploded")

	health := poller.Health()
	assert.True(t, health.Healthy, "one failure is below the threshold")
	assert.Equal(t, 1, health.ConsecutiveFailures)

	require.Error(t, poller.PollOnce(context.Background()))
	assert.False(t, poller.Health().Healthy, "sustained failure is unhealthy")

	assert.Equal(t, []string{EventFleetUpdated, EventFleetError, EventFleetError}, publisher.all())
}

func TestPollOnce_RecoversAfterFailure(t *testing.T) {
	clock := newFakeClock(baseTime)
	provider := &fakeProvider{err: errors.New("down")}
	cache := NewSnapshotCache()

	poller := NewPoller(PollerDeps{
		Provider: provider, Cache: cache, Logger: quietLogger(), Clock: clock, Interval: time.Second,
	})
	require.Error(t, poller.PollOnce(context.Background()))

	provider.set([]domain.Device{deviceAt("d1", 32.7, -117.1, domain.DriveStatusIdle, baseTime)}, nil)
	clock.Advance(time.Second)
	require.NoError(t, poller.PollOnce(context.Background()))

	snapshot := cache.Get()
	assert.False(t, snapshot.Stale, "a successful poll clears staleness")
	assert.Empty(t, snapshot.Error)
	assert.Zero(t, poller.Health().ConsecutiveFailures)
}

func TestRecordHistory_OnlyStoresMeaningfulChanges(t *testing.T) {
	clock := newFakeClock(baseTime)
	provider := &fakeProvider{}
	history := &fakeHistory{}
	cache := NewSnapshotCache()

	poller := NewPoller(PollerDeps{
		Provider: provider, Cache: cache, History: history,
		Logger: quietLogger(), Clock: clock, Interval: time.Second,
		HistoryEnabled: true, HistoryMinDistance: 15,
	})


	provider.set([]domain.Device{deviceAt("d1", 32.700000, -117.100000, domain.DriveStatusDriving, baseTime)}, nil)
	require.NoError(t, poller.PollOnce(context.Background()))
	require.Len(t, history.all(), 1)

	
	provider.set([]domain.Device{deviceAt("d1", 32.700010, -117.100010, domain.DriveStatusDriving, baseTime)}, nil)
	require.NoError(t, poller.PollOnce(context.Background()))
	assert.Len(t, history.all(), 1, "sub-threshold movement is noise")


	provider.set([]domain.Device{deviceAt("d1", 32.705000, -117.100000, domain.DriveStatusDriving, baseTime)}, nil)
	require.NoError(t, poller.PollOnce(context.Background()))
	assert.Len(t, history.all(), 2)


	provider.set([]domain.Device{deviceAt("d1", 32.705000, -117.100000, domain.DriveStatusOff, baseTime)}, nil)
	require.NoError(t, poller.PollOnce(context.Background()))
	require.Len(t, history.all(), 3)
	assert.Equal(t, domain.DriveStatusOff, history.all()[2].DriveStatus)
}

func TestRecordHistory_SkipsDevicesWithoutAFix(t *testing.T) {
	clock := newFakeClock(baseTime)
	history := &fakeHistory{}
	provider := &fakeProvider{devices: []domain.Device{deviceAt("d1", 0, 0, domain.DriveStatusOff, baseTime)}}

	poller := NewPoller(PollerDeps{
		Provider: provider, Cache: NewSnapshotCache(), History: history,
		Logger: quietLogger(), Clock: clock, Interval: time.Second, HistoryEnabled: true,
	})
	require.NoError(t, poller.PollOnce(context.Background()))
	assert.Empty(t, history.all())
}

func TestRun_StopsOnContextCancellation(t *testing.T) {
	provider := &fakeProvider{}
	poller := NewPoller(PollerDeps{
		Provider: provider, Cache: NewSnapshotCache(),
		Logger: quietLogger(), Clock: domain.SystemClock{}, Interval: 20 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		poller.Run(ctx)
		close(done)
	}()


	time.Sleep(70 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("poller did not stop when its context was cancelled")
	}
	assert.GreaterOrEqual(t, provider.callCount(), 2)
}

func TestSnapshotCache_MarkStalePreservesDevices(t *testing.T) {
	cache := NewSnapshotCache()
	cache.Set(domain.Snapshot{Devices: []domain.Device{{ID: "a"}}, FetchedAt: baseTime})

	cache.MarkStale("provider timeout")

	snapshot := cache.Get()
	assert.True(t, snapshot.Stale)
	assert.Equal(t, "provider timeout", snapshot.Error)
	require.Len(t, snapshot.Devices, 1)
	assert.Equal(t, baseTime, snapshot.FetchedAt)
}

func TestSnapshotCache_ConcurrentAccessIsSafe(t *testing.T) {
	cache := NewSnapshotCache()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); cache.Set(domain.Snapshot{FetchedAt: time.Now()}) }()
		go func() { defer wg.Done(); _ = cache.Get() }()
	}
	wg.Wait()
	assert.False(t, cache.Get().Empty())
}
