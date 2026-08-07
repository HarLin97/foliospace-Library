package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"time"
)

const contentHashWorkerPollInterval = 250 * time.Millisecond

// StartContentHashWorker starts one deliberately serialized worker. Hashing is
// streamed from disk and the database is touched only while claiming/completing
// a file, so list and manifest requests remain responsive on NAS hardware.
func (s *Service) StartContentHashWorker(ctx context.Context) {
	go func() {
		for {
			processed, err := s.processNextContentHash()
			if err != nil {
				// Keep the service alive and let the next poll retry pending work.
				time.Sleep(contentHashWorkerPollInterval)
				continue
			}
			if !processed {
				select {
				case <-ctx.Done():
					return
				case <-time.After(contentHashWorkerPollInterval):
				}
			}
		}
	}()
}

func (s *Service) processNextContentHash() (bool, error) {
	item, claimed, err := s.store.NextContentHashWork()
	if err != nil || !claimed {
		return claimed, err
	}
	info, err := os.Stat(item.Path)
	if err != nil {
		return true, s.store.FailContentHash(item.ID, err.Error())
	}
	if !info.Mode().IsRegular() {
		return true, s.store.FailContentHash(item.ID, "path is not a regular file")
	}
	if info.Size() != item.Size || !info.ModTime().Equal(item.MTime) {
		return true, s.store.ResetContentHash(item.ID, info.Size(), info.ModTime())
	}

	file, err := os.Open(item.Path)
	if err != nil {
		return true, s.store.FailContentHash(item.ID, err.Error())
	}
	hasher := sha256.New()
	_, copyErr := io.Copy(hasher, file)
	closeErr := file.Close()
	if copyErr != nil {
		return true, s.store.FailContentHash(item.ID, copyErr.Error())
	}
	if closeErr != nil {
		return true, s.store.FailContentHash(item.ID, closeErr.Error())
	}
	endInfo, err := os.Stat(item.Path)
	if err != nil {
		return true, s.store.FailContentHash(item.ID, err.Error())
	}
	if endInfo.Size() != item.Size || !endInfo.ModTime().Equal(item.MTime) {
		return true, s.store.ResetContentHash(item.ID, endInfo.Size(), endInfo.ModTime())
	}

	hash := hex.EncodeToString(hasher.Sum(nil))
	return true, s.store.CompleteContentHash(item.ID, hash)
}

// RetryFailedContentHashes makes failed work eligible for a later background
// pass without forcing a full library scan.
func (s *Service) RetryFailedContentHashes() error {
	return s.store.RetryFailedContentHashes()
}
