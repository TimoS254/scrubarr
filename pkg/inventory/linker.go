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
