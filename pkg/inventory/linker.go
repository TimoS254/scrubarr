package inventory

import "github.com/almanac1631/scrubarr/pkg/domain"

type Linker interface {
	LinkMedia(media []*domain.MediaEntry, torrents []*domain.TorrentEntry) ([]LinkedMedia, error)
}

type LinkedMedia struct {
	domain.MediaMetadata
	Files []LinkedMediaFile
}

type LinkedMediaFile struct {
	domain.MediaFile
	// TorrentEntries holds every torrent containing this file, e.g. multiple entries when it is cross-seeded to
	// several trackers.
	TorrentEntries []*domain.TorrentEntry
}

// OriginalTorrentEntry returns the first added torrent entry of the file. Any other linked torrent entries are
// cross-seeds of it, which are not subject to deletion protection (tracker rules). Returns nil if the file is not
// linked to any torrent.
func (f LinkedMediaFile) OriginalTorrentEntry() *domain.TorrentEntry {
	var original *domain.TorrentEntry
	for _, torrentEntry := range f.TorrentEntries {
		if original == nil || torrentEntry.Added.Before(original.Added) {
			original = torrentEntry
		}
	}
	return original
}
