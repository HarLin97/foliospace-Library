package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"time"

	"foliospace-reader/internal/domain"
)

// ContentHashWorkItem is a primary book file waiting for a complete content hash.
// The worker must verify size and mtime again after reading because NAS files can
// be replaced while a long hash operation is in progress.
type ContentHashWorkItem struct {
	ID     int64
	BookID int64
	Path   string
	Size   int64
	MTime  time.Time
}

func (s *Store) NextContentHashWork() (ContentHashWorkItem, bool, error) {
	var item ContentHashWorkItem
	var mtime string
	row := s.db.QueryRow(`SELECT id, book_id, abs_path, size, mtime
		FROM files
		WHERE content_hash_status = 'pending'
		ORDER BY id
		LIMIT 1`)
	if err := row.Scan(&item.ID, &item.BookID, &item.Path, &item.Size, &mtime); err != nil {
		if err == sql.ErrNoRows {
			return ContentHashWorkItem{}, false, nil
		}
		return ContentHashWorkItem{}, false, err
	}
	item.MTime = parseTime(mtime)
	result, err := s.db.Exec(`UPDATE files SET content_hash_status = 'running', updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND content_hash_status = 'pending'`, item.ID)
	if err != nil {
		return ContentHashWorkItem{}, false, err
	}
	claimed, err := result.RowsAffected()
	if err != nil {
		return ContentHashWorkItem{}, false, err
	}
	return item, claimed == 1, nil
}

func (s *Store) CompleteContentHash(fileID int64, hash string) error {
	var format string
	var pageCount int
	if err := s.db.QueryRow(`SELECT b.format, b.page_count
		FROM files f JOIN books b ON b.id = f.book_id
		WHERE f.id = ?`, fileID).Scan(&format, &pageCount); err != nil {
		return err
	}
	revision, err := s.contentRevisionForFile(fileID, hash, format, pageCount)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE files
		SET content_hash = ?, content_hash_algorithm = 'sha256', content_hash_status = 'ready',
			content_hash_error = '', content_revision = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, hash, revision, fileID)
	return err
}

// ContentRevisionForHash is an opaque, deterministic identity for the source
// bytes and the currently indexed page collection. It lets clients detect a
// reader-visible page change without exposing internal database details.
func ContentRevisionForHash(hash string, format string, pageCount int) string {
	payload := "foliospace-content-revision-v1\x00" + strings.TrimSpace(hash) + "\x00" + strings.TrimSpace(format) + "\x00" + strconv.Itoa(pageCount)
	revision := sha256.Sum256([]byte(payload))
	return "sha256:" + hex.EncodeToString(revision[:])
}

// ContentRevisionForPages extends the revision with the ordered page entry
// names. A changed page collection therefore invalidates an offline copy even
// when the total page count stays the same.
func ContentRevisionForPages(hash string, format string, pages []domain.Page) string {
	ordered := append([]domain.Page(nil), pages...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Index != ordered[j].Index {
			return ordered[i].Index < ordered[j].Index
		}
		return ordered[i].Name < ordered[j].Name
	})
	manifestHasher := sha256.New()
	for _, page := range ordered {
		_, _ = manifestHasher.Write([]byte(strconv.Itoa(page.Index)))
		_, _ = manifestHasher.Write([]byte{'\x00'})
		_, _ = manifestHasher.Write([]byte(page.Name))
		_, _ = manifestHasher.Write([]byte{'\x00'})
	}
	manifestHash := hex.EncodeToString(manifestHasher.Sum(nil))
	payload := "foliospace-content-revision-v2\x00" + strings.TrimSpace(hash) + "\x00" + strings.TrimSpace(format) + "\x00" + manifestHash
	revision := sha256.Sum256([]byte(payload))
	return "sha256:" + hex.EncodeToString(revision[:])
}

func (s *Store) contentRevisionForFile(fileID int64, hash string, format string, pageCount int) (string, error) {
	rows, err := s.db.Query(`SELECT page_index, entry_name FROM pages WHERE book_id = (SELECT book_id FROM files WHERE id = ?) ORDER BY page_index, entry_name`, fileID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	pages := make([]domain.Page, 0, pageCount)
	for rows.Next() {
		var page domain.Page
		if err := rows.Scan(&page.Index, &page.Name); err != nil {
			return "", err
		}
		pages = append(pages, page)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(pages) == 0 {
		return ContentRevisionForHash(hash, format, pageCount), nil
	}
	return ContentRevisionForPages(hash, format, pages), nil
}

// RefreshBookContentRevision updates the derived revision after the scanner
// replaces a book's page collection. Hashing is deliberately not repeated.
func (s *Store) RefreshBookContentRevision(bookID int64) error {
	var fileID int64
	var hash string
	var format string
	var pageCount int
	if err := s.db.QueryRow(`SELECT f.id, f.content_hash, b.format, b.page_count
		FROM files f JOIN books b ON b.id = f.book_id
		WHERE f.book_id = ? AND f.content_hash <> ''
		ORDER BY f.id LIMIT 1`, bookID).Scan(&fileID, &hash, &format, &pageCount); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	revision, err := s.contentRevisionForFile(fileID, hash, format, pageCount)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE files SET content_revision = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND content_hash = ?`, revision, fileID, hash)
	return err
}

func (s *Store) FailContentHash(fileID int64, message string) error {
	_, err := s.db.Exec(`UPDATE files
		SET content_hash_status = 'failed', content_hash_error = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, message, fileID)
	return err
}

func (s *Store) ResetContentHash(fileID int64, size int64, mtime time.Time) error {
	_, err := s.db.Exec(`UPDATE files
		SET size = ?, mtime = ?, content_hash = '', content_hash_algorithm = '',
			content_hash_status = 'pending', content_hash_error = '', content_revision = '',
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, size, mtime.Format(time.RFC3339Nano), fileID)
	return err
}

func (s *Store) RetryFailedContentHashes() error {
	_, err := s.db.Exec(`UPDATE files SET content_hash_status = 'pending', content_hash_error = '', updated_at = CURRENT_TIMESTAMP
		WHERE content_hash_status = 'failed'`)
	return err
}
