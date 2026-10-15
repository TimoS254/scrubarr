package inventory

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/almanac1631/scrubarr/pkg/domain"
)

func getLoggerAndFilepath(manager domain.CachedManager) (*slog.Logger, string) {
	cacheDir := os.Getenv("SCRUBARR_CACHE_DIR")
	if cacheDir == "" {
		cacheDir = "./cache"
	}
	logger := slog.With("manager", fmt.Sprintf("%T", manager))
	sha1Name := sha1.Sum([]byte(fmt.Sprintf("%T", manager)))
	retrieverHash := hex.EncodeToString(sha1Name[:])
	filePath := filepath.Join(cacheDir, retrieverHash)
	return logger, filePath
}

func (s *Service) loadManagerCacheFromDisk(manager domain.CachedManager) error {
	logger, filePath := getLoggerAndFilepath(manager)
	logger.Info("Using cache manager to refresh cache")
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()
	return manager.LoadCache(file)
}

func (s *Service) saveManagerCacheToDisk(manager domain.CachedManager) error {
	logger, filePath := getLoggerAndFilepath(manager)
	cacheDir := filepath.Dir(filePath)
	if err := os.Mkdir(cacheDir, 0777); err != nil && !os.IsExist(err) {
		return fmt.Errorf("could not create cache directory (%s): %w", cacheDir, err)
	}
	logger.Info("Saving cache manager to disk")
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()
	return manager.SaveCache(file)
}

func (s *Service) RefreshCache() error {
	s.Lock()
	defer s.Unlock()
	errChan := make(chan error)
	defer close(errChan)
	refreshManagerCache := func(manager domain.CachedManager) {
		var err error
		defer func() {
			errChan <- err
		}()
		if s.useCache {
			err = s.loadManagerCacheFromDisk(manager)
			return
		}
		err = manager.RefreshCache()
		if err != nil {
			return
		}
		if s.saveCache {
			err = s.saveManagerCacheToDisk(manager)
			return
		}
	}
	go refreshManagerCache(s.mediaSourceManager)
	go refreshManagerCache(s.torrentSourceManager)
	err := <-errChan
	if refreshErr := <-errChan; refreshErr != nil {
		if err != nil {
			err = errors.Join(err, refreshErr)
		} else {
			err = refreshErr
		}
	}
	if err != nil {
		return fmt.Errorf("refresh cache failed: %w", err)
	}

	media, err := s.mediaSourceManager.GetMedia()
	if err != nil {
		return fmt.Errorf("unable to get media: %w", err)
	}
	torrents, err := s.torrentSourceManager.GetTorrents()
	if err != nil {
		return fmt.Errorf("unable to get torrents: %w", err)
	}
	linkedMediaList, err := s.linker.LinkMedia(media, torrents)
	if err != nil {
		return fmt.Errorf("unable to link media with torrents: %w", err)
	}

	s.enrichedLinkedMediaCache = make([]enrichedLinkedMedia, len(linkedMediaList))
	for i, linkedMedia := range linkedMediaList {
		evaluationReport, err := s.retentionPolicy.Evaluate(linkedMedia)
		if err != nil {
			return fmt.Errorf("unable to evaluate retention policy: %w", err)
		}
		s.enrichedLinkedMediaCache[i] = enrichedLinkedMedia{
			linkedMedia:      linkedMedia,
			evaluationReport: evaluationReport,
			size:             getSize(linkedMedia),
			added:            getAdded(linkedMedia),
		}
	}

	uniqueTorrentId := func(client string, id string) string {
		return client + "|" + id
	}

	usedTorrentKeys := make(map[string]struct{})
	for _, lm := range linkedMediaList {
		for _, file := range lm.Files {
			for _, torrentEntry := range file.TorrentEntries {
				usedTorrentKeys[uniqueTorrentId(torrentEntry.Client, torrentEntry.Id)] = struct{}{}
			}
		}
	}
	s.orphanedTorrentsCache = nil
	for _, t := range torrents {
		if _, ok := usedTorrentKeys[uniqueTorrentId(t.Client, t.Id)]; !ok {
			size := int64(0)
			for _, f := range t.Files {
				size += f.Size
			}
			decision, tracker, err := s.retentionPolicy.EvaluateTorrentEntry(t)
			if err != nil {
				return fmt.Errorf("unable to evaluate orphaned torrent entry: %w", err)
			}
			s.orphanedTorrentsCache = append(s.orphanedTorrentsCache, enrichedOrphanedTorrent{
				torrentEntry: t,
				size:         size,
				decision:     decision,
				tracker:      tracker,
			})
		}
	}
	return nil
}
