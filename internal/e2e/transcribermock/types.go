package transcribermock

import (
	"sync"
	"time"
)

// TranscriptionTask represents a transcription task
type transcriptionTask struct {
	status         string
	responseFileID string
	fileID         string
	createdAt      time.Time
}

// MockStats holds statistics for the mock server
type MockStats struct {
	mu               sync.RWMutex
	TokenRequests    int `json:"token_requests"`
	FileUploads      int `json:"file_uploads"`
	RecognitionTasks int `json:"recognition_tasks"`
	StatusChecks     int `json:"status_checks"`
	ResultDownloads  int `json:"result_downloads"`
}

// Response types

type transcriberUploadResponse struct {
	Result struct {
		RequestFileID string `json:"request_file_id"`
	} `json:"result"`
}

type transcriberRecognizeRequest struct {
	Options struct {
		Model                    string `json:"model"`
		AudioEncoding            string `json:"audio_encoding"`
		SampleRate               int    `json:"sample_rate"`
		ChannelsCount            int    `json:"channels_count"`
		SpeakerSeparationOptions struct {
			Enable                bool `json:"enable"`
			EnableOnlyMainSpeaker bool `json:"enable_only_main_speaker"`
			Count                 int  `json:"count"`
		} `json:"speaker_separation_options"`
	} `json:"options"`
	RequestFileID string `json:"request_file_id"`
}

type transcriberRecognizeResponse struct {
	Result struct {
		ID string `json:"id"`
	} `json:"result"`
}

type transcriberStatusResponse struct {
	Result struct {
		Status         string `json:"status"`
		ResponseFileID string `json:"response_file_id"`
	} `json:"result"`
}