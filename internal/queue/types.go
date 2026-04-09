package queue

import (
	"fmt"
	"io"
	"time"
)

type TranscriberTask struct {
	ID          string
	UserID      int64
	AudioData   io.ReadCloser
	AudioFormat string
	FileID      string
	CreatedAt   time.Time
}

type TranscriberResult struct {
	TaskID        string
	UserID        int64
	FileID        string
	Transcription string
	Error         error
	CreatedAt     time.Time
}

type LLMTaskType string

const (
	LLMTaskTypeSummarize LLMTaskType = "summarize"
	LLMTaskTypeChat      LLMTaskType = "chat"
	LLMTaskTypeEmbedding LLMTaskType = "embedding"
)

type LLMTask struct {
	ID        string
	UserID    int64
	TaskType  LLMTaskType
	Text      string
	Query     string // For chat tasks
	CreatedAt time.Time
}

type LLMResult struct {
	TaskID    string
	UserID    int64
	Response  string
	Error     error
	CreatedAt time.Time
}

type EmbeddingTask struct {
	ID        string
	UserID    int64
	Text      string
	FileID    string
	CreatedAt time.Time
}

type EmbeddingResult struct {
	TaskID    string
	UserID    int64
	Embedding []float32
	Error     error
	CreatedAt time.Time
}

// GenerateMessageID generates a unique message ID using timestamp
func GenerateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}
