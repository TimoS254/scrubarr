package inventory

import (
	"io"
	"testing"
	"time"

	"github.com/almanac1631/scrubarr/internal/app/webserver"
	"github.com/almanac1631/scrubarr/pkg/domain"
	"github.com/almanac1631/scrubarr/pkg/util"
	"github.com/stretchr/testify/require"
)

var testTorrentInformationMissing = webserver.TorrentInformation{
	LinkStatus: webserver.TorrentLinkMissing,
	Tracker:    domain.Tracker{},
	Ratio:      -1.0,
	Age:        time.Duration(-1),
}

func Test_generateRawFileBasedMediaRow(t *testing.T) {
	torrentEntry1 := &domain.TorrentEntry{
		Ratio: 2.6,
		Added: util.MustParseDate("2022-08-12 00:00:00"),
	}
	torrentEntry2 := &domain.TorrentEntry{
		Ratio: 0.9,
		Added: util.MustParseDate("2023-08-11 00:00:00"),
	}
	now = func() time.Time {
		return util.MustParseDate("2023-08-12 00:00:00")
	}
	type args struct {
		media enrichedLinkedMedia
	}
	tests := []struct {
		name string
		args args
		want webserver.MediaRow
	}{
		{
			name: "media entry with single complete file",
			args: args{
				media: enrichedLinkedMedia{
					linkedMedia: LinkedMedia{
						MediaMetadata: domain.MediaMetadata{
							Id:    1337,
							Type:  domain.MediaTypeMovie,
							Title: "Some movie title",
							Url:   "http://example.com/movie.mp4",
							Added: util.MustParseDate("2021-08-12 00:00:00"),
						},
						Files: []LinkedMediaFile{
							{
								MediaFile: domain.MediaFile{
									Id:               10,
									OriginalFilePath: "some/path/anywhere/Some-movie-title-1080p.mp4",
									Size:             int64(8232),
								},
								TorrentEntries: []*domain.TorrentEntry{{
									Ratio: 1.4,
									Added: util.MustParseDate("2022-08-12 00:00:00"),
								}},
							},
						},
					},
					size:  3827948,
					added: util.MustParseDate("2021-08-12 00:00:00"),
				},
			},
			want: webserver.MediaRow{
				Id:    "movie-1337",
				Type:  domain.MediaTypeMovie,
				Title: "Some movie title",
				Url:   "http://example.com/movie.mp4",
				Size:  3827948,
				Added: util.MustParseDate("2021-08-12 00:00:00"),
				TorrentInformation: webserver.TorrentInformation{
					LinkStatus: webserver.TorrentLinkPresent,
					Ratio:      1.4,
					Age:        time.Hour * 24 * 365,
				},
				ChildMediaRows: []webserver.MediaRow{{
					Id:    "movie-1337-10",
					Title: "Some-movie-title-1080p.mp4",
					Size:  int64(8232),
					Added: util.MustParseDate("2022-08-12 00:00:00"),
					TorrentInformation: webserver.TorrentInformation{
						LinkStatus: webserver.TorrentLinkPresent,
						Ratio:      1.4,
						Age:        time.Hour * 24 * 365,
					},
					ChildMediaRows: []webserver.MediaRow{},
				}},
			},
		},
		{
			name: "media entry with multiple complete files",
			args: args{
				enrichedLinkedMedia{
					size:  9000,
					added: util.MustParseDate("2020-08-12 00:00:00"),
					linkedMedia: LinkedMedia{
						MediaMetadata: domain.MediaMetadata{
							Id:    1337,
							Type:  domain.MediaTypeSeries,
							Title: "Breaking Bad",
							Url:   "https://some-series.com/series/breaking-bad",
						},
						Files: []LinkedMediaFile{
							{
								MediaFile: domain.MediaFile{
									Id:               1337_1,
									OriginalFilePath: "Breaking Bad S1 E1.mp4",
									Season:           1,
									Size:             1000,
								},
								TorrentEntries: []*domain.TorrentEntry{torrentEntry1},
							},
							{
								MediaFile: domain.MediaFile{
									Id:               1337_2,
									OriginalFilePath: "Breaking Bad S1 E2.mp4",
									Season:           1,
									Size:             1000,
								},
								TorrentEntries: []*domain.TorrentEntry{torrentEntry1},
							},
							{
								MediaFile: domain.MediaFile{
									Id:               1337_3,
									OriginalFilePath: "Breaking Bad S2 E1.mp4",
									Season:           2,
									Size:             1500,
								},
								TorrentEntries: []*domain.TorrentEntry{torrentEntry2},
							},
						},
					},
				},
			},
			want: webserver.MediaRow{
				Id:    "series-1337",
				Type:  domain.MediaTypeSeries,
				Title: "Breaking Bad",
				Url:   "https://some-series.com/series/breaking-bad",
				Size:  9000,
				Added: util.MustParseDate("2020-08-12 00:00:00"),
				TorrentInformation: webserver.TorrentInformation{
					LinkStatus: webserver.TorrentLinkPresent,
					Ratio:      -1.0,
					Age:        time.Duration(-1),
				},
				ChildMediaRows: []webserver.MediaRow{
					{
						Id:    "series-1337-13371",
						Title: "Breaking Bad S1 E1.mp4",
						Size:  1000,
						Added: util.MustParseDate("2022-08-12 00:00:00"),
						TorrentInformation: webserver.TorrentInformation{
							LinkStatus: webserver.TorrentLinkPresent,
							Ratio:      torrentEntry1.Ratio,
							Age:        now().Sub(util.MustParseDate("2022-08-12 00:00:00")),
						},
						ChildMediaRows: []webserver.MediaRow{},
					},
					{
						Id:    "series-1337-13372",
						Title: "Breaking Bad S1 E2.mp4",
						Size:  1000,
						Added: util.MustParseDate("2022-08-12 00:00:00"),
						TorrentInformation: webserver.TorrentInformation{
							LinkStatus: webserver.TorrentLinkPresent,
							Ratio:      torrentEntry1.Ratio,
							Age:        now().Sub(util.MustParseDate("2022-08-12 00:00:00")),
						},
						ChildMediaRows: []webserver.MediaRow{},
					},
					{
						Id:    "series-1337-13373",
						Title: "Breaking Bad S2 E1.mp4",
						Size:  1500,
						Added: util.MustParseDate("2023-08-11 00:00:00"),
						TorrentInformation: webserver.TorrentInformation{
							LinkStatus: webserver.TorrentLinkPresent,
							Ratio:      torrentEntry2.Ratio,
							Age:        now().Sub(util.MustParseDate("2023-08-11 00:00:00")),
						},
						ChildMediaRows: []webserver.MediaRow{},
					},
				},
			},
		},
		{
			name: "media entry with single incomplete file",
			args: args{
				media: enrichedLinkedMedia{
					linkedMedia: LinkedMedia{
						MediaMetadata: domain.MediaMetadata{
							Id:    1337,
							Type:  domain.MediaTypeMovie,
							Title: "Some movie title",
							Url:   "http://example.com/movie.mp4",
							Added: util.MustParseDate("2021-08-12 00:00:00"),
						},
						Files: []LinkedMediaFile{
							{
								MediaFile: domain.MediaFile{
									Id:               10,
									OriginalFilePath: "some/path/anywhere/Some-movie-title-1080p.mp4",
									Size:             int64(8232),
								},
								TorrentEntries: nil,
							},
						},
					},
					size:  3827948,
					added: util.MustParseDate("2021-08-12 00:00:00"),
				},
			},
			want: webserver.MediaRow{
				Id:                 "movie-1337",
				Type:               domain.MediaTypeMovie,
				Title:              "Some movie title",
				Url:                "http://example.com/movie.mp4",
				Size:               3827948,
				Added:              util.MustParseDate("2021-08-12 00:00:00"),
				TorrentInformation: testTorrentInformationMissing,
				ChildMediaRows: []webserver.MediaRow{{
					Id:                 "movie-1337-10",
					Title:              "Some-movie-title-1080p.mp4",
					Size:               int64(8232),
					TorrentInformation: testTorrentInformationMissing,
					ChildMediaRows:     []webserver.MediaRow{},
				}},
			},
		},
		{
			name: "media entry with multiple incomplete files",
			args: args{
				enrichedLinkedMedia{
					size:  9000,
					added: util.MustParseDate("2020-08-12 00:00:00"),
					linkedMedia: LinkedMedia{
						MediaMetadata: domain.MediaMetadata{
							Id:    1337,
							Type:  domain.MediaTypeSeries,
							Title: "Breaking Bad",
							Url:   "https://some-series.com/series/breaking-bad",
						},
						Files: []LinkedMediaFile{
							{
								MediaFile: domain.MediaFile{
									Id:               1337_1,
									OriginalFilePath: "Breaking Bad S1 E1.mp4",
									Season:           1,
									Size:             1000,
								},
								TorrentEntries: []*domain.TorrentEntry{torrentEntry1},
							},
							{
								MediaFile: domain.MediaFile{
									Id:               1337_2,
									OriginalFilePath: "Breaking Bad S1 E2.mp4",
									Season:           1,
									Size:             1000,
								},
								TorrentEntries: nil,
							},
							{
								MediaFile: domain.MediaFile{
									Id:               1337_3,
									OriginalFilePath: "Breaking Bad S2 E1.mp4",
									Season:           2,
									Size:             1500,
								},
								TorrentEntries: []*domain.TorrentEntry{torrentEntry2},
							},
						},
					},
				},
			},
			want: webserver.MediaRow{
				Id:    "series-1337",
				Type:  domain.MediaTypeSeries,
				Title: "Breaking Bad",
				Url:   "https://some-series.com/series/breaking-bad",
				Size:  9000,
				Added: util.MustParseDate("2020-08-12 00:00:00"),
				TorrentInformation: webserver.TorrentInformation{
					LinkStatus: webserver.TorrentLinkIncomplete,
					Ratio:      -1.0,
					Age:        time.Duration(-1),
				},
				ChildMediaRows: []webserver.MediaRow{
					{
						Id:    "series-1337-13371",
						Title: "Breaking Bad S1 E1.mp4",
						Size:  1000,
						Added: util.MustParseDate("2022-08-12 00:00:00"),
						TorrentInformation: webserver.TorrentInformation{
							LinkStatus: webserver.TorrentLinkPresent,
							Ratio:      torrentEntry1.Ratio,
							Age:        now().Sub(util.MustParseDate("2022-08-12 00:00:00")),
						},
						ChildMediaRows: []webserver.MediaRow{},
					},
					{
						Id:                 "series-1337-13372",
						Title:              "Breaking Bad S1 E2.mp4",
						Size:               1000,
						Added:              time.Time{},
						TorrentInformation: testTorrentInformationMissing,
						ChildMediaRows:     []webserver.MediaRow{},
					},
					{
						Id:    "series-1337-13373",
						Title: "Breaking Bad S2 E1.mp4",
						Size:  1500,
						Added: util.MustParseDate("2023-08-11 00:00:00"),
						TorrentInformation: webserver.TorrentInformation{
							LinkStatus: webserver.TorrentLinkPresent,
							Ratio:      torrentEntry2.Ratio,
							Age:        now().Sub(util.MustParseDate("2023-08-11 00:00:00")),
						},
						ChildMediaRows: []webserver.MediaRow{},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateRawMediaRowFromLinkedMedia(tt.args.media)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_applyEvaluationReport(t *testing.T) {
	tracker := &domain.Tracker{
		Name:     "mock-tracker",
		MinRatio: 0.0,
		MinAge:   time.Duration(0),
	}
	tracker2 := &domain.Tracker{
		Name:     "mock-tracker-2",
		MinRatio: 1.0,
		MinAge:   time.Hour * 24 * 30,
	}
	torrentEntry1 := &domain.TorrentEntry{
		Id:    "some-torrent-entry-1",
		Added: util.MustParseDate("2020-08-12 00:00:00"),
	}
	torrentEntry2 := &domain.TorrentEntry{
		Id:    "some-torrent-entry-2",
		Added: util.MustParseDate("2020-08-13 00:00:00"),
	}
	torrentInfoPresent1 := webserver.TorrentInformation{
		LinkStatus: webserver.TorrentLinkPresent,
		Ratio:      1.0,
		Age:        time.Hour * 24 * 30,
	}
	torrentInfoPresentTracker1 := torrentInfoPresent1
	torrentInfoPresentTracker1.Tracker = *tracker
	torrentInfoPresent2 := webserver.TorrentInformation{
		LinkStatus: webserver.TorrentLinkPresent,
		Tracker:    *tracker,
		Ratio:      2.5,
		Age:        time.Hour * 24 * 15,
	}
	torrentInfoPresentTracker2 := torrentInfoPresent2
	torrentInfoPresentTracker2.Tracker = *tracker
	type args struct {
		media enrichedLinkedMedia
		row   webserver.MediaRow
	}
	tests := []struct {
		name string
		args args
		want webserver.MediaRow
	}{
		{
			name: "single file movie",
			args: args{
				media: enrichedLinkedMedia{
					linkedMedia: LinkedMedia{
						MediaMetadata: domain.MediaMetadata{
							Id: 10,
						},
						Files: []LinkedMediaFile{
							{
								MediaFile: domain.MediaFile{
									Id: 1337,
								},
							},
						},
					},
					evaluationReport: EvaluationReport{
						Result:  EvaluationReportPart{Decision: domain.DecisionSafeToDelete},
						Seasons: map[int]EvaluationReportPart{},
						Files: map[int64]EvaluationReportPart{
							1337: {Decision: domain.DecisionSafeToDelete},
						},
					},
				},
				row: webserver.MediaRow{
					Id: "movie-10",
					TorrentInformation: webserver.TorrentInformation{
						LinkStatus: webserver.TorrentLinkPresent,
					},
					ChildMediaRows: []webserver.MediaRow{
						{
							Id: "movie-10-1337",
							TorrentInformation: webserver.TorrentInformation{
								LinkStatus: webserver.TorrentLinkPresent,
							},
						},
					},
				},
			},
			want: webserver.MediaRow{
				Id:       "movie-10",
				Decision: domain.DecisionSafeToDelete,
				TorrentInformation: webserver.TorrentInformation{
					LinkStatus: webserver.TorrentLinkPresent,
				},
				AllowDeletion: true,
				ChildMediaRows: []webserver.MediaRow{{
					Id: "movie-10-1337",
					TorrentInformation: webserver.TorrentInformation{
						LinkStatus: webserver.TorrentLinkPresent,
					},
					Decision:      domain.DecisionSafeToDelete,
					AllowDeletion: true,
				}},
			},
		},
		{
			name: "multi file series",
			args: args{
				media: enrichedLinkedMedia{
					linkedMedia: LinkedMedia{
						MediaMetadata: domain.MediaMetadata{
							Id: 10,
						},
						Files: []LinkedMediaFile{
							{
								MediaFile: domain.MediaFile{
									Id:     1337_1,
									Season: 1,
								},
							},
							{
								MediaFile: domain.MediaFile{
									Id:     1337_2,
									Season: 1,
								},
							},
							{
								MediaFile: domain.MediaFile{
									Id:     1337_3,
									Season: 2,
								},
							},
							{
								MediaFile: domain.MediaFile{
									Id: 1337_4,
								},
							},
						},
					},
					evaluationReport: EvaluationReport{
						Result: EvaluationReportPart{Decision: domain.DecisionSafeToDelete},
						Seasons: map[int]EvaluationReportPart{
							1: {Decision: domain.DecisionSafeToDelete},
							2: {Decision: domain.DecisionSafeToDelete},
						},
						Files: map[int64]EvaluationReportPart{
							1337_1: {Decision: domain.DecisionSafeToDelete},
							1337_2: {Decision: domain.DecisionSafeToDelete},
							1337_3: {Decision: domain.DecisionSafeToDelete},
							1337_4: {Decision: domain.DecisionSafeToDelete},
						},
					},
				},
				row: webserver.MediaRow{
					Id:                 "series-10",
					TorrentInformation: testTorrentInformationMissing,
					ChildMediaRows: []webserver.MediaRow{
						{
							Id:                 "series-10-13371",
							TorrentInformation: testTorrentInformationMissing,
						},
						{
							Id:                 "series-10-13372",
							TorrentInformation: testTorrentInformationMissing,
						},
						{
							Id:                 "series-10-13373",
							TorrentInformation: testTorrentInformationMissing,
						},
						{
							Id:                 "series-10-13374",
							TorrentInformation: testTorrentInformationMissing,
						},
					},
				},
			},
			want: webserver.MediaRow{
				Id:                 "series-10",
				Decision:           domain.DecisionSafeToDelete,
				TorrentInformation: testTorrentInformationMissing,
				AllowDeletion:      true,
				ChildMediaRows: []webserver.MediaRow{
					{
						Id:                 "series-10-s-1",
						Title:              "Season 1",
						TorrentInformation: testTorrentInformationMissing,
						Decision:           domain.DecisionSafeToDelete,
						AllowDeletion:      true,
						ChildMediaRows: []webserver.MediaRow{
							{
								Id:                 "series-10-13371",
								TorrentInformation: testTorrentInformationMissing,
								Decision:           domain.DecisionSafeToDelete,
								AllowDeletion:      true,
							},
							{
								Id:                 "series-10-13372",
								TorrentInformation: testTorrentInformationMissing,
								Decision:           domain.DecisionSafeToDelete,
								AllowDeletion:      true,
							},
						},
					},
					{
						Id:                 "series-10-s-2",
						Title:              "Season 2",
						TorrentInformation: testTorrentInformationMissing,
						Decision:           domain.DecisionSafeToDelete,
						AllowDeletion:      true,
						ChildMediaRows: []webserver.MediaRow{
							{
								Id:                 "series-10-13373",
								TorrentInformation: testTorrentInformationMissing,
								Decision:           domain.DecisionSafeToDelete,
								AllowDeletion:      true,
							},
						},
					},
					{
						Id:                 "series-10-13374",
						TorrentInformation: testTorrentInformationMissing,
						Decision:           domain.DecisionSafeToDelete,
						AllowDeletion:      true,
					},
				},
			},
		},
		{
			name: "calc season size attribute",
			args: args{
				media: enrichedLinkedMedia{
					linkedMedia: LinkedMedia{
						MediaMetadata: domain.MediaMetadata{
							Id: 10,
						},
						Files: []LinkedMediaFile{
							{
								MediaFile: domain.MediaFile{
									Id:     1337_1,
									Season: 1,
									Size:   1000,
								},
							},
							{
								MediaFile: domain.MediaFile{
									Id:     1337_2,
									Season: 1,
									Size:   1500,
								},
							},
						},
					},
					evaluationReport: EvaluationReport{
						Result: EvaluationReportPart{
							Decision: domain.DecisionSafeToDelete,
						},
						Seasons: map[int]EvaluationReportPart{
							1: {
								Decision: domain.DecisionSafeToDelete,
							},
						},
						Files: map[int64]EvaluationReportPart{
							1337_1: {
								Decision: domain.DecisionSafeToDelete,
							},
							1337_2: {
								Decision: domain.DecisionSafeToDelete,
							},
						},
					},
				},
				row: webserver.MediaRow{
					Id:                 "series-10",
					TorrentInformation: testTorrentInformationMissing,
					ChildMediaRows: []webserver.MediaRow{
						{
							Id:                 "series-10-13371",
							TorrentInformation: testTorrentInformationMissing,
							Size:               1000,
						},
						{
							Id:                 "series-10-13372",
							TorrentInformation: testTorrentInformationMissing,
							Size:               1500,
						},
					},
				},
			},
			want: webserver.MediaRow{
				Id:                 "series-10",
				Decision:           domain.DecisionSafeToDelete,
				TorrentInformation: testTorrentInformationMissing,
				AllowDeletion:      true,
				ChildMediaRows: []webserver.MediaRow{
					{
						Id:                 "series-10-s-1",
						Title:              "Season 1",
						TorrentInformation: testTorrentInformationMissing,
						Decision:           domain.DecisionSafeToDelete,
						Size:               2500,
						AllowDeletion:      true,
						ChildMediaRows: []webserver.MediaRow{
							{
								Id:                 "series-10-13371",
								TorrentInformation: testTorrentInformationMissing,
								Size:               1000,
								Decision:           domain.DecisionSafeToDelete,
								AllowDeletion:      true,
							},
							{
								Id:                 "series-10-13372",
								TorrentInformation: testTorrentInformationMissing,
								Size:               1500,
								Decision:           domain.DecisionSafeToDelete,
								AllowDeletion:      true,
							},
						},
					},
				},
			},
		},
		{
			name: "calc season torrent attributes - same torrent",
			args: args{
				media: enrichedLinkedMedia{
					linkedMedia: LinkedMedia{
						MediaMetadata: domain.MediaMetadata{
							Id: 10,
						},
						Files: []LinkedMediaFile{
							{
								MediaFile: domain.MediaFile{
									Id:     1337_1,
									Season: 1,
								},
								TorrentEntries: []*domain.TorrentEntry{torrentEntry1},
							},
							{
								MediaFile: domain.MediaFile{
									Id:     1337_2,
									Season: 1,
								},
								TorrentEntries: []*domain.TorrentEntry{torrentEntry1},
							},
						},
					},
					evaluationReport: EvaluationReport{
						Result: EvaluationReportPart{Decision: domain.DecisionSafeToDelete},
						Seasons: map[int]EvaluationReportPart{
							1: {Decision: domain.DecisionSafeToDelete},
						},
						Files: map[int64]EvaluationReportPart{
							1337_1: {
								Decision: domain.DecisionSafeToDelete,
								Tracker:  tracker,
							},
							1337_2: {
								Decision: domain.DecisionSafeToDelete,
								Tracker:  tracker,
							},
						},
					},
				},
				row: webserver.MediaRow{
					Id:                 "series-10",
					TorrentInformation: torrentInfoPresent1,
					ChildMediaRows: []webserver.MediaRow{
						{
							Id:                 "series-10-13371",
							TorrentInformation: torrentInfoPresent1,
							Added:              util.MustParseDate("2020-08-12 00:00:00"),
						},
						{
							Id:                 "series-10-13372",
							TorrentInformation: torrentInfoPresent1,
							Added:              util.MustParseDate("2020-08-12 00:00:00"),
						},
					},
				},
			},
			want: webserver.MediaRow{
				Id:                 "series-10",
				Decision:           domain.DecisionSafeToDelete,
				TorrentInformation: torrentInfoPresentTracker1,
				AllowDeletion:      true,
				ChildMediaRows: []webserver.MediaRow{
					{
						Id:                 "series-10-s-1",
						Title:              "Season 1",
						TorrentInformation: torrentInfoPresentTracker1,
						Added:              util.MustParseDate("2020-08-12 00:00:00"),
						Decision:           domain.DecisionSafeToDelete,
						AllowDeletion:      true,
						ChildMediaRows: []webserver.MediaRow{
							{
								Id:                 "series-10-13371",
								TorrentInformation: torrentInfoPresentTracker1,
								Added:              util.MustParseDate("2020-08-12 00:00:00"),
								Decision:           domain.DecisionSafeToDelete,
							},
							{
								Id:                 "series-10-13372",
								TorrentInformation: torrentInfoPresentTracker1,
								Added:              util.MustParseDate("2020-08-12 00:00:00"),
								Decision:           domain.DecisionSafeToDelete,
							},
						},
					},
				},
			},
		},
		{
			name: "calc season torrent attributes - different torrents",
			args: args{
				media: enrichedLinkedMedia{
					linkedMedia: LinkedMedia{
						MediaMetadata: domain.MediaMetadata{
							Id: 10,
						},
						Files: []LinkedMediaFile{
							{
								MediaFile: domain.MediaFile{
									Id:     1337_1,
									Season: 1,
								},
								TorrentEntries: []*domain.TorrentEntry{torrentEntry1},
							},
							{
								MediaFile: domain.MediaFile{
									Id:     1337_2,
									Season: 1,
								},
								TorrentEntries: []*domain.TorrentEntry{torrentEntry2},
							},
						},
					},
					evaluationReport: EvaluationReport{
						Result: EvaluationReportPart{Decision: domain.DecisionPending},
						Seasons: map[int]EvaluationReportPart{
							1: {Decision: domain.DecisionPending},
						},
						Files: map[int64]EvaluationReportPart{
							1337_1: {
								Decision: domain.DecisionSafeToDelete,
								Tracker:  tracker,
							},
							1337_2: {
								Decision: domain.DecisionPending,
								Tracker:  tracker,
							},
						},
					},
				},
				row: webserver.MediaRow{
					Id: "series-10",
					TorrentInformation: webserver.TorrentInformation{
						LinkStatus: webserver.TorrentLinkPresent,
						Ratio:      -1.0,
						Age:        time.Duration(-1),
					},
					ChildMediaRows: []webserver.MediaRow{
						{
							Id:                 "series-10-13371",
							TorrentInformation: torrentInfoPresent1,
							Added:              util.MustParseDate("2020-08-12 00:00:00"),
						},
						{
							Id:                 "series-10-13372",
							TorrentInformation: torrentInfoPresent2,
							Added:              util.MustParseDate("2020-08-13 00:00:00"),
						},
					},
				},
			},
			want: webserver.MediaRow{
				Id:       "series-10",
				Decision: domain.DecisionPending,
				TorrentInformation: webserver.TorrentInformation{
					LinkStatus: webserver.TorrentLinkPresent,
					Tracker:    *tracker,
					Ratio:      -1.0,
					Age:        time.Duration(-1),
				},
				AllowDeletion: true,
				ChildMediaRows: []webserver.MediaRow{
					{
						Id:    "series-10-s-1",
						Title: "Season 1",
						TorrentInformation: webserver.TorrentInformation{
							LinkStatus: webserver.TorrentLinkPresent,
							Tracker:    *tracker,
							Ratio:      -1.0,
							Age:        time.Duration(-1),
						},
						Decision:      domain.DecisionPending,
						AllowDeletion: true,
						ChildMediaRows: []webserver.MediaRow{
							{
								Id:                 "series-10-13371",
								TorrentInformation: torrentInfoPresentTracker1,
								Added:              util.MustParseDate("2020-08-12 00:00:00"),
								Decision:           domain.DecisionSafeToDelete,
								AllowDeletion:      true,
							},
							{
								Id:                 "series-10-13372",
								TorrentInformation: torrentInfoPresentTracker2,
								Added:              util.MustParseDate("2020-08-13 00:00:00"),
								Decision:           domain.DecisionPending,
								AllowDeletion:      true,
							},
						},
					},
				},
			},
		},
		{
			name: "calc season torrent attributes - different trackers",
			args: args{
				media: enrichedLinkedMedia{
					linkedMedia: LinkedMedia{
						MediaMetadata: domain.MediaMetadata{
							Id: 10,
						},
						Files: []LinkedMediaFile{
							{
								MediaFile: domain.MediaFile{
									Id:     1337_1,
									Season: 1,
								},
								TorrentEntries: []*domain.TorrentEntry{torrentEntry1},
							},
							{
								MediaFile: domain.MediaFile{
									Id:     1337_2,
									Season: 1,
								},
								TorrentEntries: []*domain.TorrentEntry{torrentEntry2},
							},
						},
					},
					evaluationReport: EvaluationReport{
						Result: EvaluationReportPart{Decision: domain.DecisionPending},
						Seasons: map[int]EvaluationReportPart{
							1: {Decision: domain.DecisionPending},
						},
						Files: map[int64]EvaluationReportPart{
							1337_1: {
								Decision: domain.DecisionSafeToDelete,
								Tracker:  tracker,
							},
							1337_2: {
								Decision: domain.DecisionPending,
								Tracker:  tracker2,
							},
						},
					},
				},
				row: webserver.MediaRow{
					Id: "series-10",
					TorrentInformation: webserver.TorrentInformation{
						LinkStatus: webserver.TorrentLinkPresent,
						Ratio:      -1.0,
						Age:        time.Duration(-1),
					},
					ChildMediaRows: []webserver.MediaRow{
						{
							Id:                 "series-10-13371",
							TorrentInformation: torrentInfoPresent1,
							Added:              util.MustParseDate("2020-08-12 00:00:00"),
						},
						{
							Id:                 "series-10-13372",
							TorrentInformation: torrentInfoPresent2,
							Added:              util.MustParseDate("2020-08-13 00:00:00"),
						},
					},
				},
			},
			want: webserver.MediaRow{
				Id:       "series-10",
				Decision: domain.DecisionPending,
				TorrentInformation: webserver.TorrentInformation{
					LinkStatus: webserver.TorrentLinkPresent,
					Ratio:      -1.0,
					Age:        time.Duration(-1),
				},
				AllowDeletion: true,
				ChildMediaRows: []webserver.MediaRow{
					{
						Id:    "series-10-s-1",
						Title: "Season 1",
						TorrentInformation: webserver.TorrentInformation{
							LinkStatus: webserver.TorrentLinkPresent,
							Ratio:      -1.0,
							Age:        time.Duration(-1),
						},
						Decision:      domain.DecisionPending,
						AllowDeletion: true,
						ChildMediaRows: []webserver.MediaRow{
							{
								Id:                 "series-10-13371",
								TorrentInformation: torrentInfoPresentTracker1,
								Added:              util.MustParseDate("2020-08-12 00:00:00"),
								Decision:           domain.DecisionSafeToDelete,
								AllowDeletion:      true,
							},
							{
								Id: "series-10-13372",
								TorrentInformation: webserver.TorrentInformation{
									LinkStatus: webserver.TorrentLinkPresent,
									Tracker:    *tracker2,
									Ratio:      2.5,
									Age:        time.Hour * 24 * 15,
								},
								Added:         util.MustParseDate("2020-08-13 00:00:00"),
								Decision:      domain.DecisionPending,
								AllowDeletion: true,
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyEvaluationReport(tt.args.media, tt.args.row)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_getRawMediaRowFromFile_CrossSeed(t *testing.T) {
	currentTime := util.MustParseDate("2023-08-12 00:00:00")
	file := LinkedMediaFile{
		MediaFile: domain.MediaFile{Id: 10, OriginalFilePath: "movie.mp4", Size: 100},
		TorrentEntries: []*domain.TorrentEntry{
			{Ratio: 2.6, Added: util.MustParseDate("2022-08-12 00:00:00")},
			{Ratio: 0.9, Added: util.MustParseDate("2023-08-11 00:00:00")},
			{Ratio: 1.2, Added: util.MustParseDate("2023-01-01 00:00:00")},
		},
	}
	row := getRawMediaRowFromFile(currentTime, "movie-1", file)
	require.Equal(t, webserver.TorrentInformation{
		LinkStatus: webserver.TorrentLinkPresent,
		Ratio:      0.9,
		Age:        24 * time.Hour,
	}, row.TorrentInformation)
	require.Equal(t, util.MustParseDate("2023-08-11 00:00:00"), row.Added)
}

type mockMediaSourceManager struct {
	media          []*domain.MediaEntry
	deletedFileIds []int64
}

func (m *mockMediaSourceManager) RefreshCache() error             { return nil }
func (m *mockMediaSourceManager) SaveCache(_ io.Writer) error     { return nil }
func (m *mockMediaSourceManager) LoadCache(_ io.ReadSeeker) error { return nil }
func (m *mockMediaSourceManager) GetMedia() ([]*domain.MediaEntry, error) {
	return m.media, nil
}
func (m *mockMediaSourceManager) DeleteMediaFiles(_ domain.MediaType, fileIds []int64, _ bool) error {
	m.deletedFileIds = append(m.deletedFileIds, fileIds...)
	return nil
}

type mockTorrentSourceManager struct {
	torrents        []*domain.TorrentEntry
	deletedTorrents []string
}

func (m *mockTorrentSourceManager) RefreshCache() error             { return nil }
func (m *mockTorrentSourceManager) SaveCache(_ io.Writer) error     { return nil }
func (m *mockTorrentSourceManager) LoadCache(_ io.ReadSeeker) error { return nil }
func (m *mockTorrentSourceManager) GetTorrents() ([]*domain.TorrentEntry, error) {
	return m.torrents, nil
}
func (m *mockTorrentSourceManager) DeleteTorrent(client string, id string) error {
	m.deletedTorrents = append(m.deletedTorrents, client+"|"+id)
	return nil
}

// mockLinker links every media file to all torrents with the same name as the file path.
type mockLinker struct{}

func (mockLinker) LinkMedia(media []*domain.MediaEntry, torrents []*domain.TorrentEntry) ([]LinkedMedia, error) {
	linkedMediaList := make([]LinkedMedia, 0)
	for _, mediaEntry := range media {
		linkedMedia := LinkedMedia{MediaMetadata: mediaEntry.MediaMetadata}
		for _, file := range mediaEntry.Files {
			linkedFile := LinkedMediaFile{MediaFile: file}
			for _, torrent := range torrents {
				if torrent.Name == file.OriginalFilePath {
					linkedFile.TorrentEntries = append(linkedFile.TorrentEntries, torrent)
				}
			}
			linkedMedia.Files = append(linkedMedia.Files, linkedFile)
		}
		linkedMediaList = append(linkedMediaList, linkedMedia)
	}
	return linkedMediaList, nil
}

type mockRetentionPolicy struct{}

func (mockRetentionPolicy) Evaluate(_ LinkedMedia) (EvaluationReport, error) {
	return EvaluationReport{Result: EvaluationReportPart{Decision: domain.DecisionSafeToDelete}}, nil
}

func (mockRetentionPolicy) EvaluateTorrentEntry(_ *domain.TorrentEntry) (domain.Decision, *domain.Tracker, error) {
	return domain.DecisionSafeToDelete, nil, nil
}

func TestService_CrossSeed(t *testing.T) {
	mediaManager := &mockMediaSourceManager{media: []*domain.MediaEntry{{
		MediaMetadata: domain.MediaMetadata{Id: 1, Type: domain.MediaTypeMovie, Title: "Some movie"},
		Files:         []domain.MediaFile{{Id: 11, Season: -1, OriginalFilePath: "movie.mp4", Size: 100}},
	}}}
	torrentManager := &mockTorrentSourceManager{torrents: []*domain.TorrentEntry{
		{Client: "deluge", Id: "original", Name: "movie.mp4"},
		{Client: "deluge", Id: "cross-seed", Name: "movie.mp4"},
		{Client: "rtorrent", Id: "cross-seed-other-client", Name: "movie.mp4"},
		{Client: "deluge", Id: "orphan", Name: "other.mp4"},
	}}
	service := NewService(false, false, mediaManager, torrentManager, mockLinker{}, mockRetentionPolicy{})
	require.NoError(t, service.RefreshCache())

	// cross-seeds must not be reported as orphaned torrents
	require.Len(t, service.orphanedTorrentsCache, 1)
	require.Equal(t, "orphan", service.orphanedTorrentsCache[0].torrentEntry.Id)

	// deleting the media removes all cross-seeded torrents
	require.NoError(t, service.DeleteMedia("movie-1"))
	require.Equal(t, []string{"deluge|original", "deluge|cross-seed", "rtorrent|cross-seed-other-client"}, torrentManager.deletedTorrents)
	require.Equal(t, []int64{11}, mediaManager.deletedFileIds)
}
