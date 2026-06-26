package model

import "time"

type Chat struct {
	ID       int64
	Username string
	Title    string
	AddedAt  time.Time
}

type Message struct {
	ID     int64
	ChatID int64
	Sender string
	Text   string
	SentAt time.Time
}

type DigestCursor struct {
	UserID     int64
	ChatID     int64
	LastReadAt time.Time
}
