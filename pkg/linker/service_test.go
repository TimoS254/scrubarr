package linker

import (
	"reflect"
	"testing"

	"github.com/almanac1631/scrubarr/pkg/domain"
	"github.com/almanac1631/scrubarr/pkg/inventory"
	"github.com/almanac1631/scrubarr/pkg/util"
)

func TestService_LinkMedia(t *testing.T) {
	mediaMetaData := domain.MediaMetadata{
		Id:    1337,
		Type:  domain.MediaTypeMovie,
		Title: "Some movie",
		Url:   "https://somemovie.com",
		Added: util.MustParseDate("2026-02-10 13:25:07"),
	}
	mediaFile := domain.MediaFile{
		Id:               13371,
		Season:           -1,
		OriginalFilePath: "Some Movie.mp4",
		Size:             913829,
	}
	mediaEntry := domain.MediaEntry{
		MediaMetadata: mediaMetaData,
		Files:         []domain.MediaFile{mediaFile},
	}

	torrentEntryNoMatch := domain.TorrentEntry{
		Client: "mock-client",
		Id:     "stc-1",
		Name:   "Some Differently Named Movie.mp4",
		Ratio:  1.2,
		Added:  util.MustParseDate("2026-02-12 08:14:07"),
		Trackers: []string{
			"mock-tracker",
		},
		Files: []*domain.TorrentFile{
			{
				Path: "movies/nice-ones/Some Movie/Some Differently Named Movie.mp4",
				Size: 47473377,
			},
		},
	}

	torrentEntry := torrentEntryNoMatch
	torrentEntry.Name = "Some Movie.mp4"

	torrentEntryWithoutExt := torrentEntryNoMatch
	torrentEntryWithoutExt.Name = "Some Movie"

	torrentEntryOnlyFileMatch := torrentEntryNoMatch
	torrentEntryOnlyFileMatch.Files = []*domain.TorrentFile{{
		"Some Movie.mp4",
		913829,
	}}

	torrentEntryOnlyFileMatchWithFullPath := torrentEntryNoMatch
	torrentEntryOnlyFileMatchWithFullPath.Files = []*domain.TorrentFile{{
		"movies/nice-ones/Some Movie/Some Movie.mp4",
		913829,
	}}

	torrentEntryNoMatchWrongFileSize := torrentEntryOnlyFileMatch
	torrentEntryNoMatchWrongFileSize.Files = []*domain.TorrentFile{{
		torrentEntryOnlyFileMatch.Files[0].Path,
		10,
	}}

	torrentEntryCrossSeed := torrentEntry
	torrentEntryCrossSeed.Id = "stc-2"
	torrentEntryCrossSeed.Trackers = []string{"mock-tracker-2"}

	type args struct {
		media    []*domain.MediaEntry
		torrents []*domain.TorrentEntry
	}
	tests := []struct {
		name    string
		args    args
		want    []inventory.LinkedMedia
		wantErr bool
	}{
		{
			"media not linked",
			args{
				[]*domain.MediaEntry{&mediaEntry},
				[]*domain.TorrentEntry{&torrentEntryNoMatch},
			},
			[]inventory.LinkedMedia{{mediaMetaData,
				[]inventory.LinkedMediaFile{{mediaFile, nil}},
			}},
			false,
		},
		{
			"media linked with torrent entry - name match",
			args{
				[]*domain.MediaEntry{&mediaEntry},
				[]*domain.TorrentEntry{&torrentEntry},
			},
			[]inventory.LinkedMedia{{
				mediaMetaData,
				[]inventory.LinkedMediaFile{{mediaFile, []*domain.TorrentEntry{&torrentEntry}}},
			}},
			false,
		},
		{
			"media linked with torrent entry - without file extension",
			args{
				[]*domain.MediaEntry{&mediaEntry},
				[]*domain.TorrentEntry{&torrentEntryWithoutExt},
			},
			[]inventory.LinkedMedia{{
				mediaMetaData,
				[]inventory.LinkedMediaFile{{mediaFile, []*domain.TorrentEntry{&torrentEntryWithoutExt}}},
			}},
			false,
		},
		{
			"media linked with torrent entry - only file match",
			args{
				[]*domain.MediaEntry{&mediaEntry},
				[]*domain.TorrentEntry{&torrentEntryOnlyFileMatch},
			},
			[]inventory.LinkedMedia{{
				mediaMetaData,
				[]inventory.LinkedMediaFile{{mediaFile, []*domain.TorrentEntry{&torrentEntryOnlyFileMatch}}},
			}},
			false,
		},
		{
			"media linked with torrent entry - only file match with full file path",
			args{
				[]*domain.MediaEntry{&mediaEntry},
				[]*domain.TorrentEntry{&torrentEntryOnlyFileMatchWithFullPath},
			},
			[]inventory.LinkedMedia{{
				mediaMetaData,
				[]inventory.LinkedMediaFile{{mediaFile, []*domain.TorrentEntry{&torrentEntryOnlyFileMatchWithFullPath}}},
			}},
			false,
		},
		{
			"media linked with all cross-seeded torrent entries",
			args{
				[]*domain.MediaEntry{&mediaEntry},
				[]*domain.TorrentEntry{&torrentEntry, &torrentEntryNoMatch, &torrentEntryCrossSeed},
			},
			[]inventory.LinkedMedia{{
				mediaMetaData,
				[]inventory.LinkedMediaFile{{mediaFile, []*domain.TorrentEntry{&torrentEntry, &torrentEntryCrossSeed}}},
			}},
			false,
		},
		{
			"media not linked - file match with wrong size",
			args{
				[]*domain.MediaEntry{&mediaEntry},
				[]*domain.TorrentEntry{&torrentEntryNoMatchWrongFileSize},
			},
			[]inventory.LinkedMedia{{
				mediaMetaData,
				[]inventory.LinkedMediaFile{{mediaFile, nil}},
			}},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Service{}
			got, err := s.LinkMedia(tt.args.media, tt.args.torrents)
			if (err != nil) != tt.wantErr {
				t.Errorf("LinkMedia() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LinkMedia() got = %v, want %v", got, tt.want)
			}
		})
	}
}
