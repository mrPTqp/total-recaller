package models

import "time"

// Meeting represents a meeting record in database
type Meeting struct {
	ID         int       `json:"id"`
	TelegramID int64     `json:"telegram_id"`
	AudioURL   string    `json:"audio_url"`
	FullText   string    `json:"full_text"`
	Summary    string    `json:"summary"`
	CreatedAt  time.Time `json:"created_at"`
}

// MeetingSearchResult represents a search result
type MeetingSearchResult struct {
	ID        int       `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

// TimeNow returns current time
func TimeNow() time.Time {
	return time.Now()
}
