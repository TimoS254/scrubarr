package retentionpolicy

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/almanac1631/scrubarr/pkg/domain"
	"github.com/almanac1631/scrubarr/pkg/inventory"
)

var _ inventory.RetentionPolicy = (*Service)(nil)

var now = time.Now

type Service struct {
	trackerResolver TrackerResolver
}

func NewService(trackerResolver TrackerResolver) *Service {
	return &Service{trackerResolver: trackerResolver}
}

func (s Service) Evaluate(media inventory.LinkedMedia) (inventory.EvaluationReport, error) {
	globalDecision := domain.DecisionSafeToDelete
	files := make(map[int64]inventory.EvaluationReportPart)
	for _, linkedMediaFile := range media.Files {
		// a file is only safe to delete if every linked torrent (e.g. cross-seeds) satisfies its tracker rules
		var tracker *domain.Tracker
		safeToDelete := true
		for i, torrentEntry := range linkedMediaFile.TorrentEntries {
			torrentTracker, err := s.trackerResolver.Resolve(torrentEntry.Trackers)
			if errors.Is(err, ErrTrackerNotFound) {
				slog.Warn("tracker not found for linked media file", "linkedMediaFile", linkedMediaFile, "torrentEntry", torrentEntry)
			} else if err != nil {
				return inventory.EvaluationReport{}, fmt.Errorf("could not resolve tracker for linked media file (%+v): %w", linkedMediaFile, err)
			}

			if torrentTracker == nil || !isTorrentEntrySafeToDelete(torrentEntry, torrentTracker) {
				safeToDelete = false
			}
			// only report a tracker if all linked torrents share the same one
			if i == 0 {
				tracker = torrentTracker
			} else if tracker != torrentTracker {
				tracker = nil
			}
		}
		decision := domain.DecisionSafeToDelete
		if !safeToDelete {
			decision = domain.DecisionPending
			globalDecision = domain.DecisionPending
		}
		files[linkedMediaFile.Id] = inventory.EvaluationReportPart{
			Decision: decision,
			Tracker:  tracker,
		}
	}
	var seasons map[int]inventory.EvaluationReportPart = nil
	if media.Type == domain.MediaTypeSeries {
		seasons = make(map[int]inventory.EvaluationReportPart)
		for _, linkedMediaFile := range media.Files {
			if linkedMediaFile.Season == -1 {
				continue
			}
			existingReport, ok := seasons[linkedMediaFile.Season]
			fileReport := files[linkedMediaFile.Id]
			if !ok {
				seasons[linkedMediaFile.Season] = inventory.EvaluationReportPart{
					Decision: fileReport.Decision,
				}
			} else {
				if existingReport.Decision == domain.DecisionSafeToDelete && fileReport.Decision != domain.DecisionSafeToDelete {
					existingReport.Decision = fileReport.Decision
					seasons[linkedMediaFile.Season] = existingReport
				}
				if existingReport.Tracker != fileReport.Tracker {
					existingReport.Tracker = nil
				}
			}
		}
	}
	return inventory.EvaluationReport{
		Result: inventory.EvaluationReportPart{
			Decision: globalDecision,
		},
		Seasons: seasons,
		Files:   files,
	}, nil
}

func (s Service) EvaluateTorrentEntry(torrent *domain.TorrentEntry) (domain.Decision, *domain.Tracker, error) {
	tracker, err := s.trackerResolver.Resolve(torrent.Trackers)
	if errors.Is(err, ErrTrackerNotFound) {
		return domain.DecisionPending, nil, nil
	}
	if err != nil {
		slog.Warn("tracker not found for torrent entry", "torrentEntry", torrent)
		return domain.DecisionPending, nil, nil
	}
	if isTorrentEntrySafeToDelete(torrent, tracker) {
		return domain.DecisionSafeToDelete, tracker, nil
	}
	return domain.DecisionPending, tracker, nil
}

func isTorrentEntrySafeToDelete(torrentEntry *domain.TorrentEntry, tracker *domain.Tracker) bool {
	if torrentEntry.Ratio < tracker.MinRatio {
		return false
	}
	if torrentEntry.Added.Add(tracker.MinAge).After(now()) {
		return false
	}
	return true
}
