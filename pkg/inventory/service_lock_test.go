package inventory

import (
	"io"
	"testing"
	"time"

	"github.com/almanac1631/scrubarr/internal/app/webserver"
	"github.com/almanac1631/scrubarr/pkg/domain"
	"github.com/stretchr/testify/require"
)

type stubCachedManager struct{}

func (stubCachedManager) RefreshCache() error                  { return nil }
func (stubCachedManager) SaveCache(io.Writer) error            { return nil }
func (stubCachedManager) LoadCache(reader io.ReadSeeker) error { return nil }

type stubMediaSourceManager struct{ stubCachedManager }

func (stubMediaSourceManager) GetMedia() ([]*domain.MediaEntry, error) { return nil, nil }
func (stubMediaSourceManager) DeleteMediaFiles(domain.MediaType, []int64, bool) error {
	return nil
}

type stubTorrentSourceManager struct{ stubCachedManager }

func (stubTorrentSourceManager) GetTorrents() ([]*domain.TorrentEntry, error) { return nil, nil }
func (stubTorrentSourceManager) DeleteTorrent(string, string) error           { return nil }

type stubLinker struct{}

func (stubLinker) LinkMedia([]*domain.MediaEntry, []*domain.TorrentEntry) ([]LinkedMedia, error) {
	return nil, nil
}

type stubRetentionPolicy struct{}

func (stubRetentionPolicy) Evaluate(LinkedMedia) (EvaluationReport, error) {
	return EvaluationReport{}, nil
}
func (stubRetentionPolicy) EvaluateTorrentEntry(*domain.TorrentEntry) (domain.Decision, *domain.Tracker, error) {
	return domain.DecisionPending, nil, nil
}

func newStubService() *Service {
	return NewService(false, false, stubMediaSourceManager{}, stubTorrentSourceManager{}, stubLinker{}, stubRetentionPolicy{})
}

// requireReturnsWithin fails the test if fn blocks, e.g. because of a lock upgrade deadlock.
func requireReturnsWithin(t *testing.T, timeout time.Duration, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		fn()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatal("call did not return in time (deadlock?)")
	}
}

func TestService_GetMediaInventory_LoadsEmptyCacheWithoutDeadlock(t *testing.T) {
	service := newStubService()
	sortInfo := webserver.SortInfo{Key: webserver.SortKeyName, Order: webserver.SortOrderAsc}
	var rows []webserver.MediaRow
	var hasNext bool
	var err error
	requireReturnsWithin(t, 2*time.Second, func() {
		rows, hasNext, err = service.GetMediaInventory(1, sortInfo)
	})
	require.NoError(t, err)
	require.False(t, hasNext)
	require.Empty(t, rows)
	require.NotNil(t, service.enrichedLinkedMediaCache)
}

func TestService_GetOrphanedTorrents_LoadsEmptyCacheWithoutDeadlock(t *testing.T) {
	service := newStubService()
	sortInfo := webserver.SortInfo{Key: webserver.SortKeyName, Order: webserver.SortOrderAsc}
	var rows []webserver.OrphanedTorrentRow
	var hasNext bool
	var err error
	requireReturnsWithin(t, 2*time.Second, func() {
		rows, hasNext, err = service.GetOrphanedTorrents(1, sortInfo)
	})
	require.NoError(t, err)
	require.False(t, hasNext)
	require.Empty(t, rows)
	require.NotNil(t, service.enrichedLinkedMediaCache)
}
