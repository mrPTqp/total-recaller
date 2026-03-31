package telegrammock

import (
	"sync"

	tele "gopkg.in/telebot.v3"
)

// MockStats tracks statistics about the mock server.
type MockStats struct {
	UpdatesSent     int            `json:"updates_sent"`
	FilesServed     int            `json:"files_served"`
	PollingRequests int            `json:"polling_requests"`
	EventsByType    map[string]int `json:"events_by_type"`
	mu              sync.RWMutex
}

func (m *TelegramAPIMock) recordEventType(update Update) {
	m.stats.mu.Lock()
	defer m.stats.mu.Unlock()

	if update.Message == nil {
		return
	}

	msg := update.Message
	switch {
	case msg.Voice != nil:
		m.stats.EventsByType["voice"]++
	case msg.Audio != nil:
		m.stats.EventsByType["audio"]++
	case len(msg.Entities) > 0 && msg.Entities[0].Type == tele.EntityCommand:
		m.stats.EventsByType["command"]++
	case msg.Text != "":
		m.stats.EventsByType["text"]++
	}
}