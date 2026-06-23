package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	_ "modernc.org/sqlite"
)

type Storage struct {
	conn   *sql.DB
	logger *slog.Logger
}

func NewStorage(path string, logger *slog.Logger) (*Storage, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return &Storage{conn: conn, logger: logger}, nil
}

func (s *Storage) Init(ctx context.Context) error {
	query := `CREATE TABLE IF NOT EXISTS chats (
		id       INTEGER PRIMARY KEY,
		username TEXT NOT NULL,
		title    TEXT NOT NULL,
		added_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS messages (
		id      INTEGER,
		chat_id INTEGER NOT NULL REFERENCES chats(id),
		sender  TEXT NOT NULL,
		text    TEXT NOT NULL,
		sent_at DATETIME NOT NULL,
		PRIMARY KEY (id, chat_id)
	);

	CREATE TABLE IF NOT EXISTS digest_cursors (
		user_id      INTEGER NOT NULL,
		chat_id      INTEGER NOT NULL REFERENCES chats(id),
		last_read_at DATETIME NOT NULL,
		PRIMARY KEY (user_id, chat_id)
	);`

	if _, err := s.conn.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("init tables: %w", err)
	}

	s.logger.Info("storage initialized", "tables", "chats, messages, digest_cursors")
	return nil
}
