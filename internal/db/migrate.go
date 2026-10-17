package db

import "database/sql"

func Migrate(conn *sql.DB) error {
	statements := []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE IF NOT EXISTS content_reviews (id TEXT PRIMARY KEY, player_id TEXT NOT NULL REFERENCES players(id) ON DELETE CASCADE, kind TEXT NOT NULL, album_id TEXT NOT NULL DEFAULT '', caption TEXT NOT NULL DEFAULT '', status TEXT NOT NULL, created_at INTEGER NOT NULL, trace_id TEXT UNIQUE, result TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS auth_sessions (
			token TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			created_at TEXT NOT NULL,
			expires_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS player_sessions (
			token TEXT PRIMARY KEY,
			team_id TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
			player_id TEXT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
			created_at TEXT NOT NULL,
			expires_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS teams (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS players (
			id TEXT PRIMARY KEY,
			team_id TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			number INTEGER,
			positions_json TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS wechat_members (
            player_id TEXT PRIMARY KEY REFERENCES players(id) ON DELETE CASCADE,
            app_id TEXT NOT NULL,
            open_id TEXT NOT NULL,
            role TEXT NOT NULL DEFAULT 'member' CHECK(role IN ('member','coach','owner')),
            profile_complete INTEGER NOT NULL DEFAULT 0,
            avatar_file_name TEXT NOT NULL DEFAULT '',
            UNIQUE(app_id, open_id)
        )`,
		`CREATE TABLE IF NOT EXISTS events (
			id TEXT PRIMARY KEY,
			team_id TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			starts_at TEXT NOT NULL,
			ends_at TEXT NOT NULL DEFAULT '',
			location TEXT NOT NULL,
			opponent TEXT NOT NULL,
			notes TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS event_locations (
			id TEXT PRIMARY KEY,
			team_id TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(team_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS attendance_records (
			event_id TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
			player_id TEXT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
			status TEXT NOT NULL,
			note TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (event_id, player_id)
		)`,
		`CREATE TABLE IF NOT EXISTS tactic_boards (
			id TEXT PRIMARY KEY,
			team_id TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			format INTEGER NOT NULL,
			formation TEXT NOT NULL,
			slots_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS event_cancellations (
			event_id TEXT PRIMARY KEY REFERENCES events(id) ON DELETE CASCADE,
			cancelled_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS terms (
			id TEXT PRIMARY KEY,
			team_id TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			starts_on TEXT NOT NULL,
			ends_on TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(team_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS special_activities (
			id TEXT PRIMARY KEY,
			team_id TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			starts_on TEXT NOT NULL,
			ends_on TEXT NOT NULL,
			summary TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS special_activity_entries (
			id TEXT PRIMARY KEY,
			special_activity_id TEXT NOT NULL REFERENCES special_activities(id) ON DELETE CASCADE,
			title TEXT NOT NULL,
			occurs_at TEXT NOT NULL,
			location TEXT NOT NULL,
			opponent TEXT NOT NULL,
			home_score INTEGER,
			away_score INTEGER,
			notes TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS albums (
			id TEXT PRIMARY KEY,
			team_id TEXT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
			event_id TEXT REFERENCES events(id) ON DELETE RESTRICT,
			special_activity_id TEXT REFERENCES special_activities(id) ON DELETE RESTRICT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			CHECK ((event_id IS NOT NULL AND special_activity_id IS NULL) OR (event_id IS NULL AND special_activity_id IS NOT NULL)),
			UNIQUE(event_id),
			UNIQUE(special_activity_id)
		)`,
		`CREATE TABLE IF NOT EXISTS photos (
			id TEXT PRIMARY KEY,
			album_id TEXT NOT NULL REFERENCES albums(id) ON DELETE CASCADE,
			uploader_player_id TEXT REFERENCES players(id) ON DELETE RESTRICT,
			uploader_role TEXT NOT NULL,
			caption TEXT NOT NULL,
			file_name TEXT NOT NULL,
			thumbnail_file_name TEXT NOT NULL,
			content_type TEXT NOT NULL,
			width INTEGER NOT NULL,
			height INTEGER NOT NULL,
			byte_size INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			deleted_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_attendance_event_player ON attendance_records(event_id, player_id)`,
		`CREATE INDEX IF NOT EXISTS idx_terms_team_dates ON terms(team_id, starts_on, ends_on)`,
		`CREATE INDEX IF NOT EXISTS idx_special_activities_team_dates ON special_activities(team_id, starts_on, ends_on)`,
		`CREATE INDEX IF NOT EXISTS idx_photos_album_created ON photos(album_id, created_at)`,
	}

	for _, statement := range statements {
		if _, err := conn.Exec(statement); err != nil {
			return err
		}
	}
	for _, column := range []struct {
		table      string
		name       string
		definition string
	}{
		{"tactic_boards", "template_id", "TEXT NOT NULL DEFAULT ''"},
		{"tactic_boards", "opponent_template_id", "TEXT NOT NULL DEFAULT ''"},
		{"tactic_boards", "opponent_formation", "TEXT NOT NULL DEFAULT ''"},
		{"events", "ends_at", "TEXT NOT NULL DEFAULT ''"},
		{"events", "format", "INTEGER NOT NULL DEFAULT 0"},
		{"wechat_members", "avatar_file_name", "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := ensureColumn(conn, column.table, column.name, column.definition); err != nil {
			return err
		}
	}
	return nil
}

func ensureColumn(conn *sql.DB, table, name, definition string) error {
	exists, err := columnExists(conn, table, name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = conn.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + name + ` ` + definition)
	return err
}

func columnExists(conn *sql.DB, table, name string) (bool, error) {
	rows, err := conn.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var columnName string
		var columnType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, err
		}
		if columnName == name {
			return true, nil
		}
	}
	return false, rows.Err()
}
