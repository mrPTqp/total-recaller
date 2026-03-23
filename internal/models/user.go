package models

import "time"

// User represents a user in the database
type User struct {
	ID        int       `json:"id"`
	TelegramID int64    `json:"telegram_id"`
	Username  string    `json:"username"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	CreatedAt time.Time `json:"created_at"`
}