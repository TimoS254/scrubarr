package linker

import (
	"path/filepath"

	"github.com/almanac1631/scrubarr/pkg/domain"
	"github.com/almanac1631/scrubarr/pkg/inventory"
)

var _ inventory.Linker = (*Service)(nil)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s Service) LinkMedia(mediaEntries []*domain.MediaEntry, torrentEntries []*domain.TorrentEntry) ([]inventory.LinkedMedia, error) {
	linkedMedias := make([]inventory.LinkedMedia, 0)
	for _, mediaEntry := range mediaEntries {
		linkedMedia := &inventory.LinkedMedia{
			MediaMetadata: mediaEntry.MediaMetadata,
		}
		for _, mediaFile := range mediaEntry.Files {
			linkedMediaFile := inventory.LinkedMediaFile{
				MediaFile:      mediaFile,
				TorrentEntries: searchLinkedTorrentEntries(mediaFile, torrentEntries),
			}
			linkedMedia.Files = append(linkedMedia.Files, linkedMediaFile)
		}
		if linkedMedia.Files != nil {
			linkedMedias = append(linkedMedias, *linkedMedia)
		}
	}
	return linkedMedias, nil
}

// searchLinkedTorrentEntries returns all torrent entries containing the given media file. Multiple entries are
// returned if the file is cross-seeded to several trackers.
func searchLinkedTorrentEntries(mediaFile domain.MediaFile, torrentEntries []*domain.TorrentEntry) []*domain.TorrentEntry {
	var linkedTorrentEntries []*domain.TorrentEntry
	for _, torrentEntry := range torrentEntries {
		if isTorrentEntryMatching(mediaFile, torrentEntry) {
			linkedTorrentEntries = append(linkedTorrentEntries, torrentEntry)
		}
	}
	return linkedTorrentEntries
}

func isTorrentEntryMatching(mediaFile domain.MediaFile, torrentEntry *domain.TorrentEntry) bool {
	if torrentEntry.Name == mediaFile.OriginalFilePath {
		return true
	}

	torrentEntryNameWithExt := torrentEntry.Name + filepath.Ext(mediaFile.OriginalFilePath)
	if torrentEntryNameWithExt == mediaFile.OriginalFilePath {
		return true
	}

	for _, torrentEntryFile := range torrentEntry.Files {
		if torrentEntryFile.Size != mediaFile.Size {
			continue
		}
		if torrentEntryFile.Path == mediaFile.OriginalFilePath {
			return true
		}
		torrentFileBase := filepath.Base(torrentEntryFile.Path)
		if torrentFileBase == mediaFile.OriginalFilePath {
			return true
		}
	}
	return false
}
