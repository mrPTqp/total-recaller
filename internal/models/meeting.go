package models

import "time"

// Meeting represents a meeting record in database
type Meeting struct {
	ID         int       `json:"id" db:"id"`
	TelegramID int64     `json:"telegram_id" db:"telegram_id"`
	FileId     string    `json:"file_id" db:"file_id"`
	FullText   string    `json:"full_text" db:"full_text"`
	Summary    string    `json:"summary" db:"summary"`
	Embedding  Vector    `json:"embedding,omitempty" db:"embedding"` 
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

type MeetingSearchResult struct {
	ID        int       `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func TimeNow() time.Time {
	return time.Now()
}
