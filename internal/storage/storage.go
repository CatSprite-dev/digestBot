package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/CatSprite-dev/digestBot/internal/model"
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
		username TEXT,
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

func (s *Storage) ChatExists(ctx context.Context, chatID int64) (bool, error) {
	query := "SELECT EXISTS(SELECT 1 FROM chats WHERE id = ?)"
	row := s.conn.QueryRowContext(ctx, query, chatID)
	var exist bool
	err := row.Scan(&exist)
	if err != nil {
		return false, fmt.Errorf("query: %w", err)
	}
	return exist, nil
}

func (s *Storage) UpsertChat(ctx context.Context, chat model.Chat) error {
	query := `INSERT INTO chats (id, username, title, added_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		username = excluded.username,
		title = excluded.title`

	if _, err := s.conn.ExecContext(ctx, query, chat.ID, chat.UserName, chat.Title, chat.AddedAt); err != nil {
		return fmt.Errorf("upsert chat: %w", err)
	}
	s.logger.Info("chat added", "chat", chat.Title)
	return nil
}

func (s *Storage) SaveMessage(ctx context.Context, message model.Message) error {
	query := `INSERT INTO messages (id, chat_id, sender, text, sent_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id, chat_id) DO NOTHING`

	if _, err := s.conn.ExecContext(ctx, query, message.ID, message.ChatID, message.Sender, message.Text, message.SentAt); err != nil {
		return fmt.Errorf("save message: %w", err)
	}

	s.logger.Info("message saved", "from", message.Sender, "date", message.SentAt)
	return nil
}

func (s *Storage) GetMessagesSince(ctx context.Context, chatID int64, since time.Time) ([]model.Message, error) {
	messages := []model.Message{}

	query := `SELECT id, chat_id, sender, text, sent_at 
		FROM messages 
		WHERE chat_id = ? AND sent_at > ?
		ORDER BY sent_at ASC`

	rows, err := s.conn.QueryContext(ctx, query, chatID, since)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.ID, &m.ChatID, &m.Sender, &m.Text, &m.SentAt); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		messages = append(messages, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}

	s.logger.Debug("fetched messages", "chat_id", chatID, "count", len(messages), "since", since)
	return messages, nil
}

func (s *Storage) GetCursor(ctx context.Context, userID, chatID int64) (time.Time, error) {
	query := `SELECT last_read_at FROM digest_cursors WHERE user_id = ? AND chat_id = ?`

	var t time.Time
	err := s.conn.QueryRowContext(ctx, query, userID, chatID).Scan(&t)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("get cursor: %w", err)
	}

	return t, nil
}

func (s *Storage) SetCursor(ctx context.Context, userID, chatID int64, t time.Time) error {
	query := `INSERT INTO digest_cursors (user_id, chat_id, last_read_at)
		VALUES (?, ?, ?)
		ON CONFLICT(user_id, chat_id) DO UPDATE SET
			last_read_at = excluded.last_read_at`

	if _, err := s.conn.ExecContext(ctx, query, userID, chatID, t); err != nil {
		return fmt.Errorf("set cursor: %w", err)
	}
	s.logger.Info("cursor set successful", "chat", chatID, "time", t)
	return nil
}
