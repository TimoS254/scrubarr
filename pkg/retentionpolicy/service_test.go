package retentionpolicy

import (
	"testing"
	"time"

	"github.com/almanac1631/scrubarr/pkg/domain"
	"github.com/almanac1631/scrubarr/pkg/inventory"
	"github.com/almanac1631/scrubarr/pkg/util"
	"github.com/stretchr/testify/assert"
)

type mockTrackerResolver struct {
	tracker *domain.Tracker
}

func (m mockTrackerResolver) Resolve(_ []string) (*domain.Tracker, error) {
	return m.tracker, nil
}

func TestService_Evaluate(t *testing.T) {
	mediaMetadata := domain.MediaMetadata{
		Id:    1337,
		Type:  domain.MediaTypeMovie,
		Title: "Some Movie",
		Added: time.Now(),
	}
	mediaMetaDataSeasons := mediaMetadata
	mediaMetaDataSeasons.Title = "Some Series"
	mediaMetaDataSeasons.Type = domain.MediaTypeSeries

	tracker := domain.Tracker{
		Name:     "mockTracker",
		MinRatio: 0,
		MinAge:   0,
	}

	linkedMediaFile := inventory.LinkedMediaFile{
		MediaFile: domain.MediaFile{
			Id:     13371,
			Season: -1,
		},
		TorrentEntries: []*domain.TorrentEntry{{
			Ratio: 0,
			Added: util.MustParseDate("2025-12-16 13:14:15"),
		}},
	}
	linkedMediaFileNoTorrent := linkedMediaFile
	linkedMediaFileNoTorrent.TorrentEntries = nil

	// season 1 is safe to delete (1) and pending (2)
	linkedMediaFileSeason1E1 := inventory.LinkedMediaFile{
		MediaFile: domain.MediaFile{
			Id:     1337_1_1,
			Season: 1,
		},
		TorrentEntries: []*domain.TorrentEntry{{
			Ratio: 0,
			// use old Added to satisfy high age tracker
			Added: util.MustParseDate("2023-12-16 13:14:15"),
		}},
	}
	linkedMediaFileSeason1E2 := inventory.LinkedMediaFile{
		MediaFile: domain.MediaFile{
			Id:     1337_1_2,
			Season: 1,
		},
		TorrentEntries: []*domain.TorrentEntry{{
			Ratio: 0,
			// use recent date to not satisfy high age tracker
			Added: util.MustParseDate("2025-12-16 13:14:15"),
		}},
	}
	// season 2 is safe to delete
	linkedMediaFileSeason2E1 := inventory.LinkedMediaFile{
		MediaFile: domain.MediaFile{
			Id:     1337_2_1,
			Season: 2,
		},
		TorrentEntries: []*domain.TorrentEntry{{
			Ratio: 0,
			// use old Added to satisfy high age tracker
			Added: util.MustParseDate("2023-12-16 13:14:15"),
		}},
	}
	// season 3 is pending
	linkedMediaFileSeason3E1 := inventory.LinkedMediaFile{
		MediaFile: domain.MediaFile{
			Id:     1337_3_1,
			Season: 3,
		},
		TorrentEntries: []*domain.TorrentEntry{{
			Ratio: 0,
			Added: util.MustParseDate("2025-12-16 13:14:15"),
		}},
	}
	// file without series
	linkedMediaFileSeasonNoSeason := inventory.LinkedMediaFile{
		MediaFile: domain.MediaFile{
			Id:     1337_1,
			Season: -1,
		},
		TorrentEntries: []*domain.TorrentEntry{{
			Ratio: 0,
			// use old Added to satisfy high age tracker
			Added: util.MustParseDate("2023-12-16 13:14:15"),
		}},
	}

	trackerHighRatio := tracker
	trackerHighRatio.MinRatio = 100

	trackerHighAge := tracker
	trackerHighAge.MinAge = time.Hour * 24 * 365

	type fields struct {
		trackerResolver TrackerResolver
	}
	type args struct {
		media inventory.LinkedMedia
	}
	now = func() time.Time {
		return util.MustParseDate("2026-02-01 13:17:09")
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    inventory.EvaluationReport
		wantErr bool
	}{
		{
			"allowed delete eval - fields complete",
			fields{mockTrackerResolver{&tracker}},
			args{
				inventory.LinkedMedia{
					MediaMetadata: mediaMetadata,
					Files:         []inventory.LinkedMediaFile{linkedMediaFile},
				},
			},
			inventory.EvaluationReport{
				Result:  inventory.EvaluationReportPart{Decision: domain.DecisionSafeToDelete},
				Seasons: nil,
				Files: map[int64]inventory.EvaluationReportPart{
					13371: {
						Decision: domain.DecisionSafeToDelete,
						Tracker:  &tracker,
					},
				},
			},
			false,
		},
		{
			"allowed delete eval - missing torrent",
			fields{mockTrackerResolver{nil}},
			args{
				inventory.LinkedMedia{
					MediaMetadata: mediaMetadata,
					Files:         []inventory.LinkedMediaFile{linkedMediaFileNoTorrent},
				},
			},
			inventory.EvaluationReport{
				Result:  inventory.EvaluationReportPart{Decision: domain.DecisionSafeToDelete},
				Seasons: nil,
				Files: map[int64]inventory.EvaluationReportPart{
					13371: {
						Decision: domain.DecisionSafeToDelete,
						Tracker:  nil,
					},
				},
			},
			false,
		},
		{
			"disallowed delete eval - ratio not fulfilled",
			fields{mockTrackerResolver{&trackerHighRatio}},
			args{
				inventory.LinkedMedia{
					MediaMetadata: mediaMetadata,
					Files:         []inventory.LinkedMediaFile{linkedMediaFile},
				},
			},
			inventory.EvaluationReport{
				Result:  inventory.EvaluationReportPart{Decision: domain.DecisionPending},
				Seasons: nil,
				Files: map[int64]inventory.EvaluationReportPart{
					13371: {
						Decision: domain.DecisionPending,
						Tracker:  &trackerHighRatio,
					},
				},
			},
			false,
		},
		{
			"disallowed delete eval - age not fulfilled",
			fields{mockTrackerResolver{&trackerHighAge}},
			args{
				inventory.LinkedMedia{
					MediaMetadata: mediaMetadata,
					Files:         []inventory.LinkedMediaFile{linkedMediaFile},
				},
			},
			inventory.EvaluationReport{
				Result:  inventory.EvaluationReportPart{Decision: domain.DecisionPending},
				Seasons: nil,
				Files: map[int64]inventory.EvaluationReportPart{
					13371: {
						Decision: domain.DecisionPending,
						Tracker:  &trackerHighAge,
					},
				},
			},
			false,
		},
		{
			"disallowed delete eval - seasons",
			fields{mockTrackerResolver{&trackerHighAge}},
			args{
				inventory.LinkedMedia{
					MediaMetadata: mediaMetaDataSeasons,
					Files: []inventory.LinkedMediaFile{
						linkedMediaFileSeason1E1,
						linkedMediaFileSeason1E2,
						linkedMediaFileSeason2E1,
						linkedMediaFileSeason3E1,
						linkedMediaFileSeasonNoSeason,
					},
				},
			},
			inventory.EvaluationReport{
				Result: inventory.EvaluationReportPart{Decision: domain.DecisionPending},
				Seasons: map[int]inventory.EvaluationReportPart{
					1: {Decision: domain.DecisionPending},
					2: {Decision: domain.DecisionSafeToDelete},
					3: {Decision: domain.DecisionPending},
				},
				Files: map[int64]inventory.EvaluationReportPart{
					linkedMediaFileSeason1E1.Id: {
						Decision: domain.DecisionSafeToDelete,
						Tracker:  &trackerHighAge,
					},
					linkedMediaFileSeason1E2.Id: {
						Decision: domain.DecisionPending,
						Tracker:  &trackerHighAge,
					},
					linkedMediaFileSeason2E1.Id: {
						Decision: domain.DecisionSafeToDelete,
						Tracker:  &trackerHighAge,
					},
					linkedMediaFileSeason3E1.Id: {
						Decision: domain.DecisionPending,
						Tracker:  &trackerHighAge,
					},
					linkedMediaFileSeasonNoSeason.Id: {
						Decision: domain.DecisionSafeToDelete,
						Tracker:  &trackerHighAge,
					},
				},
			},
			false,
		},
		{
			"different trackers in a season",
			fields{mockTrackerResolver{&trackerHighAge}},
			args{
				inventory.LinkedMedia{
					MediaMetadata: mediaMetaDataSeasons,
					Files: []inventory.LinkedMediaFile{
						linkedMediaFileSeason1E1,
						linkedMediaFileSeason1E2,
						linkedMediaFileSeason2E1,
						linkedMediaFileSeason3E1,
						linkedMediaFileSeasonNoSeason,
					},
				},
			},
			inventory.EvaluationReport{
				Result: inventory.EvaluationReportPart{
					Decision: domain.DecisionPending,
				},
				Seasons: map[int]inventory.EvaluationReportPart{
					1: {Decision: domain.DecisionPending},
					2: {Decision: domain.DecisionSafeToDelete},
					3: {Decision: domain.DecisionPending},
				},
				Files: map[int64]inventory.EvaluationReportPart{
					linkedMediaFileSeason1E1.Id: {
						Decision: domain.DecisionSafeToDelete,
						Tracker:  &trackerHighAge,
					},
					linkedMediaFileSeason1E2.Id: {
						Decision: domain.DecisionPending,
						Tracker:  &trackerHighAge,
					},
					linkedMediaFileSeason2E1.Id: {
						Decision: domain.DecisionSafeToDelete,
						Tracker:  &trackerHighAge,
					},
					linkedMediaFileSeason3E1.Id: {
						Decision: domain.DecisionPending,
						Tracker:  &trackerHighAge,
					},
					linkedMediaFileSeasonNoSeason.Id: {
						Decision: domain.DecisionSafeToDelete,
						Tracker:  &trackerHighAge,
					},
				},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Service{
				trackerResolver: tt.fields.trackerResolver,
			}
			got, err := s.Evaluate(tt.args.media)
			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

type mockMultiTrackerResolver map[string]*domain.Tracker

func (m mockMultiTrackerResolver) Resolve(trackers []string) (*domain.Tracker, error) {
	for _, tracker := range trackers {
		if resolved, ok := m[tracker]; ok {
			return resolved, nil
		}
	}
	return nil, ErrTrackerNotFound
}

func TestService_Evaluate_CrossSeed(t *testing.T) {
	now = func() time.Time {
		return util.MustParseDate("2026-02-01 13:17:09")
	}
	trackerA := &domain.Tracker{Name: "tracker-a", MinRatio: 1, MinAge: 0}
	trackerB := &domain.Tracker{Name: "tracker-b", MinRatio: 0, MinAge: time.Hour * 24 * 30}
	resolver := mockMultiTrackerResolver{"a": trackerA, "b": trackerB}

	// satisfies tracker a rules
	torrentA := &domain.TorrentEntry{Id: "a", Trackers: []string{"a"}, Ratio: 2, Added: util.MustParseDate("2025-12-01 00:00:00")}
	torrentA2 := &domain.TorrentEntry{Id: "a2", Trackers: []string{"a"}, Ratio: 1.5, Added: util.MustParseDate("2025-12-01 00:00:00")}
	// cross-seed added recently, does not satisfy tracker b min age yet
	torrentBRecent := &domain.TorrentEntry{Id: "b-recent", Trackers: []string{"b"}, Ratio: 0, Added: util.MustParseDate("2026-01-25 00:00:00")}
	torrentBOld := &domain.TorrentEntry{Id: "b-old", Trackers: []string{"b"}, Ratio: 0, Added: util.MustParseDate("2025-12-01 00:00:00")}
	torrentUnknown := &domain.TorrentEntry{Id: "unknown", Trackers: []string{"unknown"}, Ratio: 10, Added: util.MustParseDate("2020-01-01 00:00:00")}

	mediaWithTorrents := func(torrents ...*domain.TorrentEntry) inventory.LinkedMedia {
		return inventory.LinkedMedia{
			MediaMetadata: domain.MediaMetadata{Id: 1, Type: domain.MediaTypeMovie},
			Files: []inventory.LinkedMediaFile{{
				MediaFile:      domain.MediaFile{Id: 11, Season: -1},
				TorrentEntries: torrents,
			}},
		}
	}

	tests := []struct {
		name         string
		media        inventory.LinkedMedia
		wantDecision domain.Decision
		wantTracker  *domain.Tracker
	}{
		{"all cross-seeds satisfied", mediaWithTorrents(torrentA, torrentBOld), domain.DecisionSafeToDelete, nil},
		{"one cross-seed pending", mediaWithTorrents(torrentA, torrentBRecent), domain.DecisionPending, nil},
		{"pending cross-seed listed first", mediaWithTorrents(torrentBRecent, torrentA), domain.DecisionPending, nil},
		{"cross-seed with unknown tracker", mediaWithTorrents(torrentA, torrentUnknown), domain.DecisionPending, nil},
		{"same tracker for all torrents", mediaWithTorrents(torrentA, torrentA2), domain.DecisionSafeToDelete, trackerA},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Service{trackerResolver: resolver}.Evaluate(tt.media)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantDecision, got.Result.Decision)
			assert.Equal(t, inventory.EvaluationReportPart{Decision: tt.wantDecision, Tracker: tt.wantTracker}, got.Files[11])
		})
	}
}
