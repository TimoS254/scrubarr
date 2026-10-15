package inventory

import (
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/almanac1631/scrubarr/internal/app/webserver"
	"github.com/almanac1631/scrubarr/pkg/domain"
)

var now = time.Now

const pageSize = 25

type enrichedLinkedMedia struct {
	linkedMedia      LinkedMedia
	evaluationReport EvaluationReport
	size             int64
	added            time.Time
}

func (m enrichedLinkedMedia) getScore() int {
	return decisionScore(m.evaluationReport.Result.Decision)
}

type enrichedOrphanedTorrent struct {
	torrentEntry *domain.TorrentEntry
	size         int64
	decision     domain.Decision
	tracker      *domain.Tracker
}

func (e enrichedOrphanedTorrent) getScore() int {
	return decisionScore(e.decision)
}

func decisionScore(d domain.Decision) int {
	switch d {
	case domain.DecisionSafeToDelete:
		return 0
	case domain.DecisionPending:
		return 1
	default:
		return -1
	}
}

type Service struct {
	*sync.RWMutex
	useCache, saveCache      bool
	enrichedLinkedMediaCache []enrichedLinkedMedia
	orphanedTorrentsCache    []enrichedOrphanedTorrent
	mediaSourceManager       domain.MediaSourceManager
	torrentSourceManager     domain.TorrentSourceManager
	linker                   Linker
	retentionPolicy          RetentionPolicy
}

func NewService(useCache, saveCache bool, mediaSourceManager domain.MediaSourceManager, torrentSourceManager domain.TorrentSourceManager, linker Linker, retentionPolicy RetentionPolicy) *Service {
	return &Service{RWMutex: &sync.RWMutex{}, useCache: useCache, saveCache: saveCache, mediaSourceManager: mediaSourceManager, torrentSourceManager: torrentSourceManager, linker: linker, retentionPolicy: retentionPolicy}
}

func getAdded(linkedMedia LinkedMedia) time.Time {
	added := linkedMedia.Added
	for _, file := range linkedMedia.Files {
		for _, torrentEntry := range file.TorrentEntries {
			if !torrentEntry.Added.IsZero() && torrentEntry.Added.After(added) {
				added = torrentEntry.Added
			}
		}
	}
	return added
}

func getSize(linkedMedia LinkedMedia) int64 {
	size := int64(0)
	for _, file := range linkedMedia.Files {
		size += file.Size
	}
	return size
}

func (s *Service) GetMediaInventory(page int, sortInfo webserver.SortInfo) (mediaRows []webserver.MediaRow, hasNext bool, err error) {
	s.RLock()
	defer s.RUnlock()
	if s.enrichedLinkedMediaCache == nil {
		if err := s.RefreshCache(); err != nil {
			return nil, false, err
		}
	}
	enrichedLinkedMediaList := slices.Clone(s.enrichedLinkedMediaCache)
	slices.SortFunc(enrichedLinkedMediaList, func(a, b enrichedLinkedMedia) int {
		var result int
		switch sortInfo.Key {
		case webserver.SortKeyName:
			result = strings.Compare(strings.ToLower(a.linkedMedia.Title), strings.ToLower(b.linkedMedia.Title))
			break
		case webserver.SortKeySize:
			result = cmp.Compare(a.size, b.size)
			break
		case webserver.SortKeyAdded:
			result = cmp.Compare(a.added.Unix(), b.added.Unix())
			break
		case webserver.SortKeyStatus:
			result = cmp.Compare(a.getScore(), b.getScore())
			break
		default:
			slog.Error("Received unknown sort key.", "sortKey", sortInfo.Key)
			result = 0 // mark as incomparable
		}
		if sortInfo.Order == webserver.SortOrderDesc {
			result = -result
		}
		return result
	})
	if pageSize*page < len(enrichedLinkedMediaList) {
		hasNext = true
		enrichedLinkedMediaList = enrichedLinkedMediaList[pageSize*(page-1) : pageSize*page]
	} else {
		enrichedLinkedMediaList = enrichedLinkedMediaList[pageSize*(page-1):]
	}

	mediaRows = make([]webserver.MediaRow, len(enrichedLinkedMediaList))
	for i, media := range enrichedLinkedMediaList {
		mediaRow := getMediaRow(media)
		mediaRow.ChildMediaRows = []webserver.MediaRow{}
		mediaRows[i] = mediaRow
	}
	return mediaRows, hasNext, nil
}

func (s *Service) GetExpandedMediaRow(rawId string) (mediaRowExpanded webserver.MediaRow, err error) {
	s.RLock()
	defer s.RUnlock()
	id, err := parseMediaId(rawId)
	if err != nil {
		return mediaRowExpanded, err
	}
	for _, mediaIter := range s.enrichedLinkedMediaCache {
		if mediaIter.linkedMedia.Type != id.MediaType || mediaIter.linkedMedia.Id != id.Id {
			continue
		}
		return getMediaRow(mediaIter), nil
	}
	return webserver.MediaRow{}, webserver.ErrMediaNotFound
}

// getMediaRow combines the raw media row generation and evaluation report apply logic to return an enriched media row.
func getMediaRow(media enrichedLinkedMedia) webserver.MediaRow {
	row := generateRawMediaRowFromLinkedMedia(media)
	return applyEvaluationReport(media, row)
}

// getRawFileBasedMedia parses the given media and returns a webserver.MediaRow with a file list. This function does not
// respect tracker information, deletion decision or hierarchies like seasons.
func generateRawMediaRowFromLinkedMedia(media enrichedLinkedMedia) webserver.MediaRow {
	linkedMedia := media.linkedMedia
	id := fmt.Sprintf("%s-%d", linkedMedia.Type, linkedMedia.Id)
	var torrentInformation webserver.TorrentInformation
	childMediaRows := make([]webserver.MediaRow, 0)
	currentTime := now()
	for i, file := range linkedMedia.Files {
		fileMediaRow := getRawMediaRowFromFile(currentTime, id, file)

		if i == 0 && torrentInformation.LinkStatus == "" {
			torrentInformation = fileMediaRow.TorrentInformation
		} else if i > 0 && torrentInformation != fileMediaRow.TorrentInformation {
			torrentInformation = webserver.TorrentInformation{
				LinkStatus: getCombinedTorrentLinkStatus(torrentInformation.LinkStatus, fileMediaRow.TorrentInformation.LinkStatus),
				Ratio:      -1.0,
				Age:        time.Duration(-1),
			}
		}
		childMediaRows = append(childMediaRows, fileMediaRow)
	}
	if torrentInformation.LinkStatus != webserver.TorrentLinkPresent {
		torrentInformation.Ratio = -1.0
		torrentInformation.Age = time.Duration(-1)
	}
	return webserver.MediaRow{
		Id:                 id,
		Type:               linkedMedia.Type,
		Title:              linkedMedia.Title,
		Url:                linkedMedia.Url,
		Size:               media.size,
		Added:              media.added,
		TorrentInformation: torrentInformation,
		ChildMediaRows:     childMediaRows,
	}
}

// getRawMediaRowFromFile returns a new webserver.MediaRow using the passed id and file. It uses the torrent information
// but does not respect tracker information or the deletion decision.
func getRawMediaRowFromFile(currentTime time.Time, id string, file LinkedMediaFile) webserver.MediaRow {
	fileId := fmt.Sprintf("%s-%d", id, file.Id)
	fileTorrentInformation := webserver.TorrentInformation{
		LinkStatus: webserver.TorrentLinkMissing,
		Ratio:      -1.0,
		Age:        time.Duration(-1),
	}
	var added time.Time
	// for files linked to multiple torrents (cross-seeds), show the lowest ratio and the most recently added torrent
	for i, torrentEntry := range file.TorrentEntries {
		if i == 0 || torrentEntry.Ratio < fileTorrentInformation.Ratio {
			fileTorrentInformation.Ratio = torrentEntry.Ratio
		}
		if i == 0 || torrentEntry.Added.After(added) {
			added = torrentEntry.Added
		}
	}
	if len(file.TorrentEntries) > 0 {
		fileTorrentInformation.LinkStatus = webserver.TorrentLinkPresent
		fileTorrentInformation.Age = currentTime.Sub(added)
	}
	fileMediaRow := webserver.MediaRow{
		Id:                 fileId,
		Title:              path.Base(file.OriginalFilePath),
		Size:               file.Size,
		Added:              added,
		TorrentInformation: fileTorrentInformation,
		ChildMediaRows:     make([]webserver.MediaRow, 0),
	}
	return fileMediaRow
}

// applyEvaluationReport takes the media and row params the maps the evaluation report of the media param onto the given
// row param. This includes applying the season hierarchy, calculating season based attributes and adding the decision
// derived from the report. For this function to work properly, the row`s child rows and the media`s files have to be in
// the same order.
func applyEvaluationReport(media enrichedLinkedMedia, row webserver.MediaRow) webserver.MediaRow {
	row.Decision = media.evaluationReport.Result.Decision
	row.AllowDeletion = true
	childMediaRows := make([]webserver.MediaRow, 0)
	for i, mediaRow := range row.ChildMediaRows {
		file := media.linkedMedia.Files[i]
		report := media.evaluationReport.Files[file.Id]
		mediaRow.Decision = report.Decision
		mediaRow.AllowDeletion = true
		if report.Tracker != nil {
			mediaRow.TorrentInformation.Tracker = *report.Tracker
		}
		if i == 0 {
			row.TorrentInformation.Tracker = mediaRow.TorrentInformation.Tracker
		} else if row.TorrentInformation.Tracker != mediaRow.TorrentInformation.Tracker {
			row.TorrentInformation.Tracker = domain.Tracker{}
		}

		seasonId := fmt.Sprintf("%s-s-%d", row.Id, file.Season)
		if file.Season > 0 {
			seasonRowIndex := slices.IndexFunc(childMediaRows, func(row webserver.MediaRow) bool {
				return row.Id == seasonId
			})
			var seasonRow webserver.MediaRow
			if seasonRowIndex == -1 {
				seasonRow = mediaRow
				seasonRow.ChildMediaRows = []webserver.MediaRow{mediaRow}
				seasonRow.Id = seasonId
				seasonRow.Title = fmt.Sprintf("Season %d", file.Season)
				seasonReport := media.evaluationReport.Seasons[file.Season]
				seasonRow.Decision = seasonReport.Decision
				seasonRow.AllowDeletion = true
				if seasonReport.Tracker != nil {
					seasonRow.TorrentInformation.Tracker = *seasonReport.Tracker
				}
				childMediaRows = append(childMediaRows, seasonRow)
			} else {
				seasonRow = childMediaRows[seasonRowIndex]
				seasonRow.ChildMediaRows = append(seasonRow.ChildMediaRows, mediaRow)
				seasonRow.Size = seasonRow.Size + file.Size
				if seasonRow.TorrentInformation != mediaRow.TorrentInformation {
					seasonRowTracker := seasonRow.TorrentInformation.Tracker
					if seasonRowTracker != mediaRow.TorrentInformation.Tracker {
						seasonRowTracker = domain.Tracker{}
					}
					seasonRow.TorrentInformation = webserver.TorrentInformation{
						LinkStatus: getCombinedTorrentLinkStatus(seasonRow.TorrentInformation.LinkStatus, mediaRow.TorrentInformation.LinkStatus),
						Tracker:    seasonRowTracker,
						Ratio:      -1.0,
						Age:        time.Duration(-1),
					}
					seasonRow.Added = time.Time{}
				}
				childMediaRows[seasonRowIndex] = seasonRow
			}
		} else {
			childMediaRows = append(childMediaRows, mediaRow)
		}
	}
	// prevent delete button for torrent-related seasons
	for _, mediaRow := range childMediaRows {
		if len(mediaRow.ChildMediaRows) == 0 {
			continue
		}
		if mediaRow.TorrentInformation.Ratio != -1 {
			for i, childMediaRow := range mediaRow.ChildMediaRows {
				childMediaRow.AllowDeletion = false
				mediaRow.ChildMediaRows[i] = childMediaRow
			}
		}
	}
	row.ChildMediaRows = childMediaRows
	return row
}

func (s *Service) GetOrphanedTorrents(page int, sortInfo webserver.SortInfo) (rows []webserver.OrphanedTorrentRow, hasNext bool, err error) {
	s.RLock()
	defer s.RUnlock()
	if s.enrichedLinkedMediaCache == nil {
		if err := s.RefreshCache(); err != nil {
			return nil, false, err
		}
	}
	all := slices.Clone(s.orphanedTorrentsCache)
	slices.SortFunc(all, func(a, b enrichedOrphanedTorrent) int {
		var result int
		switch sortInfo.Key {
		case webserver.SortKeyName:
			result = strings.Compare(strings.ToLower(a.torrentEntry.Name), strings.ToLower(b.torrentEntry.Name))
		case webserver.SortKeySize:
			result = cmp.Compare(a.size, b.size)
		case webserver.SortKeyAdded:
			result = cmp.Compare(a.torrentEntry.Added.Unix(), b.torrentEntry.Added.Unix())
		case webserver.SortKeyStatus:
			result = cmp.Compare(a.getScore(), b.getScore())
		default:
			slog.Error("Received unknown sort key.", "sortKey", sortInfo.Key)
			result = 0
		}
		if sortInfo.Order == webserver.SortOrderDesc {
			result = -result
		}
		return result
	})
	start := pageSize * (page - 1)
	if start >= len(all) {
		return []webserver.OrphanedTorrentRow{}, false, nil
	}
	end := start + pageSize
	if end < len(all) {
		hasNext = true
	} else {
		end = len(all)
	}
	currentTime := now()
	for _, e := range all[start:end] {
		t := e.torrentEntry
		row := webserver.OrphanedTorrentRow{
			Id:            url.PathEscape(t.Client + "-" + t.Id),
			Name:          t.Name,
			Client:        t.Client,
			Ratio:         t.Ratio,
			Added:         t.Added,
			Age:           currentTime.Sub(t.Added),
			Size:          e.size,
			Decision:      e.decision,
			AllowDeletion: true,
		}
		if e.tracker != nil {
			row.Tracker = *e.tracker
		}
		rows = append(rows, row)
	}
	return rows, hasNext, nil
}

func getCombinedTorrentLinkStatus(groupStatus, entryStatus webserver.TorrentLinkStatus) webserver.TorrentLinkStatus {
	if groupStatus == webserver.TorrentLinkMissing &&
		entryStatus == webserver.TorrentLinkPresent {
		return webserver.TorrentLinkIncomplete
	}
	return entryStatus
}

func (s *Service) DeleteMedia(rawId string) error {
	s.Lock()
	defer s.Unlock()
	id, err := parseMediaId(rawId)
	if err != nil {
		return err
	}
	// retrieve entry
	entryIndex := slices.IndexFunc(s.enrichedLinkedMediaCache, func(media enrichedLinkedMedia) bool {
		return media.linkedMedia.Type == id.MediaType && media.linkedMedia.Id == id.Id
	})
	if entryIndex == -1 {
		return webserver.ErrMediaNotFound
	}
	entry := s.enrichedLinkedMediaCache[entryIndex]

	// retrieve affected file indexes
	affectedFileIndexes := id.getMatchingLinkedMediaIndexes(entry.linkedMedia.Files)
	if len(affectedFileIndexes) == 0 {
		return webserver.ErrMediaNotFound
	}

	deletedTorrentEntries := make(map[*domain.TorrentEntry]struct{})
	fileIdsToDelete := make([]int64, 0)

	// delete torrent entries, including all cross-seeds of the affected files
	for _, affectedFileIndex := range affectedFileIndexes {
		affectedFile := entry.linkedMedia.Files[affectedFileIndex]
		for _, torrentEntry := range affectedFile.TorrentEntries {
			if _, ok := deletedTorrentEntries[torrentEntry]; ok {
				continue
			}
			err = s.torrentSourceManager.DeleteTorrent(torrentEntry.Client, torrentEntry.Id)
			if errors.Is(err, domain.ErrTorrentNotFound) {
				slog.Warn("could not find torrent entry for deletion", "linkedMediaTitle", entry.linkedMedia.Title, "file", affectedFile, "torrentEntry", torrentEntry)
			} else if err != nil {
				return fmt.Errorf("could not delete torrent %s for linked media %q (file: %+v): %w", torrentEntry, entry.linkedMedia.Title, affectedFile, err)
			}
			deletedTorrentEntries[torrentEntry] = struct{}{}
		}
		fileIdsToDelete = append(fileIdsToDelete, affectedFile.Id)
	}

	// delete media files
	err = s.mediaSourceManager.DeleteMediaFiles(entry.linkedMedia.Type, fileIdsToDelete, true)
	if err != nil {
		return fmt.Errorf("could not delete media files: %w", err)
	}

	// adjust entry in cache
	if len(affectedFileIndexes) == len(entry.linkedMedia.Files) {
		s.enrichedLinkedMediaCache = append(s.enrichedLinkedMediaCache[:entryIndex], s.enrichedLinkedMediaCache[entryIndex+1:]...)
	} else {
		for counter, affectedFileIndex := range affectedFileIndexes {
			entry.linkedMedia.Files = append(entry.linkedMedia.Files[:(affectedFileIndex-counter)], entry.linkedMedia.Files[(affectedFileIndex-counter)+1:]...)
		}
		s.enrichedLinkedMediaCache[entryIndex] = entry
	}

	return nil
}

func (s *Service) DeleteOrphanedTorrent(rawId string) error {
	s.Lock()
	defer s.Unlock()
	parts := strings.SplitN(rawId, "-", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid orphaned torrent id: %q", rawId)
	}
	client, torrentId := parts[0], parts[1]

	entryIndex := slices.IndexFunc(s.orphanedTorrentsCache, func(e enrichedOrphanedTorrent) bool {
		return e.torrentEntry.Client == client && e.torrentEntry.Id == torrentId
	})
	if entryIndex == -1 {
		return webserver.ErrMediaNotFound
	}

	err := s.torrentSourceManager.DeleteTorrent(client, torrentId)
	if err != nil && !errors.Is(err, domain.ErrTorrentNotFound) {
		return fmt.Errorf("could not delete orphaned torrent %q/%q: %w", client, torrentId, err)
	}

	s.orphanedTorrentsCache = append(s.orphanedTorrentsCache[:entryIndex], s.orphanedTorrentsCache[entryIndex+1:]...)
	return nil
}
