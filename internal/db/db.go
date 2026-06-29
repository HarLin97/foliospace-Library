package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Open(configDir string) (*sql.DB, error) {
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}

	conn, err := sql.Open("sqlite", filepath.Join(configDir, "foliospace-reader.db"))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	if err := Migrate(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func Migrate(conn *sql.DB) error {
	stmts := []string{
		`PRAGMA foreign_keys = ON`,
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA journal_mode = WAL`,
		`CREATE TABLE IF NOT EXISTS libraries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			root_path TEXT NOT NULL UNIQUE,
			asset_type TEXT NOT NULL DEFAULT 'mixed',
			exclude_patterns TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS app_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS profiles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			avatar TEXT NOT NULL DEFAULT 'reader',
			color TEXT NOT NULL DEFAULT 'teal',
			is_default INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`INSERT OR IGNORE INTO profiles(id, name, is_default) VALUES(1, 'Default', 1)`,
		`CREATE TABLE IF NOT EXISTS series (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
			title TEXT NOT NULL,
			directory_path TEXT NOT NULL DEFAULT '',
			collection_type TEXT NOT NULL DEFAULT 'directory',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(library_id, title)
		)`,
		`CREATE TABLE IF NOT EXISTS books (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			series_id INTEGER NOT NULL REFERENCES series(id) ON DELETE CASCADE,
			title TEXT NOT NULL,
			creator TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			format TEXT NOT NULL,
			page_count INTEGER NOT NULL DEFAULT 0,
			cover_status TEXT NOT NULL DEFAULT 'pending',
			analyzed INTEGER NOT NULL DEFAULT 0,
			private_status TEXT NOT NULL DEFAULT '',
			favorite INTEGER NOT NULL DEFAULT 0,
			rating INTEGER NOT NULL DEFAULT 0,
			tags TEXT NOT NULL DEFAULT '',
			summary TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(series_id, title, format)
		)`,
		`CREATE TABLE IF NOT EXISTS files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
			library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
			abs_path TEXT NOT NULL UNIQUE,
			rel_path TEXT NOT NULL,
			size INTEGER NOT NULL,
			mtime TEXT NOT NULL,
			ext TEXT NOT NULL,
			hash TEXT NOT NULL DEFAULT '',
			hash_status TEXT NOT NULL DEFAULT 'pending',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS games (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
			title TEXT NOT NULL,
			platform TEXT NOT NULL DEFAULT '',
			rom_set_name TEXT NOT NULL DEFAULT '',
			region TEXT NOT NULL DEFAULT '',
			format TEXT NOT NULL,
			file_path TEXT NOT NULL UNIQUE,
			rel_path TEXT NOT NULL,
			size INTEGER NOT NULL,
			mtime TEXT NOT NULL,
			crc32 TEXT NOT NULL DEFAULT '',
			sha1 TEXT NOT NULL DEFAULT '',
			emulator_hint TEXT NOT NULL DEFAULT '',
			compatibility TEXT NOT NULL DEFAULT 'unknown',
			last_played_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS game_metadata (
			game_id INTEGER PRIMARY KEY REFERENCES games(id) ON DELETE CASCADE,
			display_title TEXT NOT NULL DEFAULT '',
			summary TEXT NOT NULL DEFAULT '',
			release_date TEXT NOT NULL DEFAULT '',
			genres TEXT NOT NULL DEFAULT '[]',
			developers TEXT NOT NULL DEFAULT '[]',
			publishers TEXT NOT NULL DEFAULT '[]',
			players TEXT NOT NULL DEFAULT '',
			rating REAL NOT NULL DEFAULT 0,
			external_links TEXT NOT NULL DEFAULT '[]',
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS game_metadata_sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
			source TEXT NOT NULL,
			source_id TEXT NOT NULL DEFAULT '',
			matched_by TEXT NOT NULL DEFAULT '',
			confidence REAL NOT NULL DEFAULT 0,
			raw_json TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(game_id, source, source_id)
		)`,
		`CREATE TABLE IF NOT EXISTS game_artwork (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
			source TEXT NOT NULL,
			kind TEXT NOT NULL,
			url TEXT NOT NULL DEFAULT '',
			cache_path TEXT NOT NULL DEFAULT '',
			width INTEGER NOT NULL DEFAULT 0,
			height INTEGER NOT NULL DEFAULT 0,
			selected INTEGER NOT NULL DEFAULT 0,
			confidence REAL NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(game_id, source, kind, url, cache_path)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_game_metadata_sources_game ON game_metadata_sources(game_id, source)`,
		`CREATE INDEX IF NOT EXISTS idx_game_artwork_game ON game_artwork(game_id, kind, selected DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_books_series_title ON books(series_id, title, id)`,
		`CREATE INDEX IF NOT EXISTS idx_books_created_id ON books(created_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_files_book ON files(book_id)`,
		`CREATE INDEX IF NOT EXISTS idx_games_updated_id ON games(updated_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_games_lower_title ON games(LOWER(title), id)`,
		`CREATE INDEX IF NOT EXISTS idx_games_lower_platform_title ON games(LOWER(platform), LOWER(title), id)`,
		`CREATE INDEX IF NOT EXISTS idx_games_lower_filters ON games(LOWER(platform), LOWER(rom_set_name), LOWER(format), LOWER(emulator_hint))`,
		`CREATE TABLE IF NOT EXISTS videos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
			title TEXT NOT NULL,
			format TEXT NOT NULL,
			file_path TEXT NOT NULL UNIQUE,
			rel_path TEXT NOT NULL,
			size INTEGER NOT NULL,
			mtime TEXT NOT NULL,
			duration_seconds REAL NOT NULL DEFAULT 0,
			width INTEGER NOT NULL DEFAULT 0,
			height INTEGER NOT NULL DEFAULT 0,
			video_codec TEXT NOT NULL DEFAULT '',
			audio_codec TEXT NOT NULL DEFAULT '',
			thumbnail_status TEXT NOT NULL DEFAULT 'placeholder',
			last_played_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS pages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
			page_index INTEGER NOT NULL,
			entry_name TEXT NOT NULL,
			UNIQUE(book_id, page_index)
		)`,
		`CREATE TABLE IF NOT EXISTS scan_jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
			status TEXT NOT NULL,
			target_path TEXT NOT NULL DEFAULT '',
			current_path TEXT NOT NULL DEFAULT '',
			discovered_files INTEGER NOT NULL DEFAULT 0,
			indexed_files INTEGER NOT NULL DEFAULT 0,
			skipped_files INTEGER NOT NULL DEFAULT 0,
			error_count INTEGER NOT NULL DEFAULT 0,
			metadata_updated_files INTEGER NOT NULL DEFAULT 0,
			reclassified_files INTEGER NOT NULL DEFAULT 0,
			started_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			finished_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS thumbnail_jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
			size TEXT NOT NULL,
			status TEXT NOT NULL,
			priority INTEGER NOT NULL DEFAULT 0,
			cache_key TEXT NOT NULL,
			cache_path TEXT NOT NULL DEFAULT '',
			content_type TEXT NOT NULL DEFAULT '',
			width INTEGER NOT NULL DEFAULT 0,
			height INTEGER NOT NULL DEFAULT 0,
			byte_size INTEGER NOT NULL DEFAULT 0,
			error_message TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			started_at TEXT NOT NULL DEFAULT '',
			finished_at TEXT NOT NULL DEFAULT '',
			UNIQUE(book_id, size, cache_key)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_thumbnail_jobs_status_priority ON thumbnail_jobs(status, priority DESC, id)`,
		`CREATE TABLE IF NOT EXISTS scan_directories (
			library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
			abs_path TEXT NOT NULL,
			mtime TEXT NOT NULL,
			has_subdirs INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(library_id, abs_path)
		)`,
		`CREATE TABLE IF NOT EXISTS job_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id INTEGER NOT NULL REFERENCES scan_jobs(id) ON DELETE CASCADE,
			level TEXT NOT NULL,
			message TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS read_progress (
			book_id INTEGER PRIMARY KEY REFERENCES books(id) ON DELETE CASCADE,
			page_index INTEGER NOT NULL,
			locator TEXT NOT NULL DEFAULT '',
			progress_fraction REAL NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS profile_read_progress (
			profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
			book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
			page_index INTEGER NOT NULL,
			locator TEXT NOT NULL DEFAULT '',
			progress_fraction REAL NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(profile_id, book_id)
		)`,
		`CREATE TABLE IF NOT EXISTS profile_read_positions (
			profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
			book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
			reader_mode TEXT NOT NULL,
			schema TEXT NOT NULL,
			page_index INTEGER NOT NULL DEFAULT 0,
			page_key TEXT NOT NULL DEFAULT '',
			page_y_offset_ratio REAL NOT NULL DEFAULT 0,
			viewport_anchor_ratio REAL NOT NULL DEFAULT 0.28,
			document_progress REAL NOT NULL DEFAULT 0,
			page_count INTEGER NOT NULL DEFAULT 0,
			content_signature TEXT NOT NULL DEFAULT '',
			payload_json TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(profile_id, book_id, reader_mode)
		)`,
		`CREATE TABLE IF NOT EXISTS book_private_states (
			profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
			book_id INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
			private_status TEXT NOT NULL DEFAULT '',
			favorite INTEGER NOT NULL DEFAULT 0,
			rating INTEGER NOT NULL DEFAULT 0,
			tags TEXT NOT NULL DEFAULT '',
			summary TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(profile_id, book_id)
		)`,
		`CREATE TABLE IF NOT EXISTS collection_private_states (
			profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
			series_id INTEGER NOT NULL REFERENCES series(id) ON DELETE CASCADE,
			favorite INTEGER NOT NULL DEFAULT 0,
			liked INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(profile_id, series_id)
		)`,
		`CREATE TABLE IF NOT EXISTS game_private_states (
			profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
			game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
			favorite INTEGER NOT NULL DEFAULT 0,
			liked INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(profile_id, game_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_profile_read_progress_profile_updated ON profile_read_progress(profile_id, updated_at DESC, book_id)`,
		`CREATE INDEX IF NOT EXISTS idx_book_private_states_profile_favorite_updated ON book_private_states(profile_id, favorite, updated_at DESC, book_id)`,
		`CREATE INDEX IF NOT EXISTS idx_book_private_states_profile_status_updated ON book_private_states(profile_id, private_status, updated_at DESC, book_id)`,
		`CREATE INDEX IF NOT EXISTS idx_videos_updated_id ON videos(updated_at DESC, id DESC)`,
		`CREATE TABLE IF NOT EXISTS manual_collections (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS manual_collection_items (
			collection_id INTEGER NOT NULL REFERENCES manual_collections(id) ON DELETE CASCADE,
			asset_type TEXT NOT NULL,
			asset_id INTEGER NOT NULL,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(collection_id, asset_type, asset_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_manual_collection_items_collection ON manual_collection_items(collection_id, created_at, asset_type, asset_id)`,
		`CREATE TABLE IF NOT EXISTS file_errors (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			library_id INTEGER NOT NULL,
			book_id INTEGER NOT NULL DEFAULT 0,
			file_id INTEGER NOT NULL DEFAULT 0,
			job_id INTEGER NOT NULL DEFAULT 0,
			path TEXT NOT NULL,
			code TEXT NOT NULL,
			message TEXT NOT NULL,
			first_seen TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_seen TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(path, code)
		)`,
		`CREATE TABLE IF NOT EXISTS client_preferences (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			locale TEXT NOT NULL DEFAULT 'zh',
			reader_page_mode TEXT NOT NULL DEFAULT 'single',
			epub_page_mode TEXT NOT NULL DEFAULT 'single',
			epub_theme TEXT NOT NULL DEFAULT 'light',
			epub_font_size INTEGER NOT NULL DEFAULT 18,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS profile_client_preferences (
			profile_id INTEGER PRIMARY KEY REFERENCES profiles(id) ON DELETE CASCADE,
			locale TEXT NOT NULL DEFAULT 'zh',
			reader_page_mode TEXT NOT NULL DEFAULT 'single',
			epub_page_mode TEXT NOT NULL DEFAULT 'single',
			epub_theme TEXT NOT NULL DEFAULT 'light',
			epub_font_size INTEGER NOT NULL DEFAULT 18,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, stmt := range stmts {
		if _, err := conn.Exec(stmt); err != nil {
			return fmt.Errorf("migrate sqlite: %w", err)
		}
	}
	if err := addColumnIfMissing(conn, "libraries", "exclude_patterns", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := addColumnIfMissing(conn, "scan_jobs", "current_path", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := addColumnIfMissing(conn, "scan_jobs", "target_path", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := addColumnIfMissing(conn, "scan_jobs", "metadata_updated_files", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := addColumnIfMissing(conn, "scan_jobs", "reclassified_files", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := addColumnIfMissing(conn, "series", "directory_path", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := addColumnIfMissing(conn, "series", "collection_type", "TEXT NOT NULL DEFAULT 'directory'"); err != nil {
		return err
	}
	if err := addColumnIfMissing(conn, "read_progress", "locator", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := addColumnIfMissing(conn, "read_progress", "progress_fraction", "REAL NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := addColumnIfMissing(conn, "books", "private_status", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := addColumnIfMissing(conn, "books", "favorite", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := addColumnIfMissing(conn, "books", "rating", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := addColumnIfMissing(conn, "books", "tags", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := addColumnIfMissing(conn, "books", "summary", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := addColumnIfMissing(conn, "books", "creator", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := addColumnIfMissing(conn, "books", "description", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := addColumnIfMissing(conn, "libraries", "asset_type", "TEXT NOT NULL DEFAULT 'mixed'"); err != nil {
		return err
	}
	if err := addColumnIfMissing(conn, "profiles", "avatar", "TEXT NOT NULL DEFAULT 'reader'"); err != nil {
		return err
	}
	if err := addColumnIfMissing(conn, "profiles", "color", "TEXT NOT NULL DEFAULT 'teal'"); err != nil {
		return err
	}
	if _, err := conn.Exec(`INSERT OR IGNORE INTO profile_read_progress(profile_id, book_id, page_index, locator, progress_fraction, updated_at)
		SELECT 1, book_id, page_index, locator, progress_fraction, updated_at FROM read_progress`); err != nil {
		return fmt.Errorf("migrate default profile progress: %w", err)
	}
	if _, err := conn.Exec(`INSERT OR IGNORE INTO book_private_states(profile_id, book_id, private_status, favorite, rating, tags, summary, updated_at)
		SELECT 1, id, private_status, favorite, rating, tags, summary, updated_at
		FROM books
		WHERE private_status <> '' OR favorite <> 0 OR rating <> 0 OR tags <> '' OR summary <> ''`); err != nil {
		return fmt.Errorf("migrate default profile private state: %w", err)
	}
	if _, err := conn.Exec(`INSERT OR IGNORE INTO profile_client_preferences(profile_id, locale, reader_page_mode, epub_page_mode, epub_theme, epub_font_size, updated_at)
		SELECT 1, locale, reader_page_mode, epub_page_mode, epub_theme, epub_font_size, updated_at FROM client_preferences WHERE id = 1`); err != nil {
		return fmt.Errorf("migrate default profile preferences: %w", err)
	}
	return nil
}

func addColumnIfMissing(conn *sql.DB, table string, column string, definition string) error {
	rows, err := conn.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var typ string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = conn.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, definition))
	return err
}
