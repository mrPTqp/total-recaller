package queue

import (
	"fmt"
	"io"
	"time"
)

// TranscriberTask represents a transcription task
type TranscriberTask struct {
	ID          string
	UserID      int64
	AudioData   io.ReadCloser
	AudioFormat string
	FileID      string
	CreatedAt   time.Time
}

// TranscriberResult represents the result of a transcription task
type TranscriberResult struct {
	TaskID      string
	UserID      int64
	FileID      string
	Transcription string
	Error       error
	CreatedAt   time.Time
}

// LLMTaskType represents the type of LLM task
type LLMTaskType string

const (
	LLMTaskTypeSummarize LLMTaskType = "summarize"
	LLMTaskTypeChat      LLMTaskType = "chat"
)

// LLMTask represents an LLM processing task
type LLMTask struct {
	ID        string
	UserID    int64
	TaskType  LLMTaskType
	Text      string
	Query     string // For chat tasks
	CreatedAt time.Time
}

// LLMResult represents the result of an LLM task
type LLMResult struct {
	TaskID    string
	UserID    int64
	Response  string
	Error     error
	CreatedAt time.Time
}

// GenerateMessageID generates a unique message ID using timestamp
func GenerateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}
