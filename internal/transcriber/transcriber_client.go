package transcriber

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mrPTqp/total-recaller/internal/config"
	"github.com/mrPTqp/total-recaller/internal/token"
	"go.uber.org/zap"
)

type TranscriberClient struct {
	config       *config.Config
	saluteClient *SaluteClient
	logger       *zap.Logger
}


func NewTranscriberClient(config *config.Config, httpClient *http.Client, tokenManager *token.TokenManager, logger *zap.Logger) *TranscriberClient {
	return &TranscriberClient{
		config:       config,
		saluteClient: NewSaluteClient(config, httpClient, tokenManager, logger),
		logger:       logger,
	}
}

func (tc *TranscriberClient) TranscribeFile(ctx context.Context, audio io.ReadCloser, audioEncoding string) (string, error) {
	// Step 1: Upload file
	requestFileID, err := tc.saluteClient.UploadFile(ctx, audio)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	// Step 2: Create recognition task
	taskID, err := tc.saluteClient.CreateRecognitionTask(ctx, requestFileID, audioEncoding)
	if err != nil {
		return "", fmt.Errorf("failed to create recognition task: %w", err)
	}

	// Step 3: Poll for completion
	for {
		status, responseFileID, err := tc.saluteClient.CheckTaskStatus(ctx, taskID)
		if err != nil {
			return "", fmt.Errorf("failed to check task status: %w", err)
		}

		switch status {
		case "DONE":
			// Step 4: Download result
			result, err := tc.saluteClient.DownloadResult(ctx, responseFileID)
			if err != nil {
				return "", fmt.Errorf("failed to download result: %w", err)
			}

			// Extract speaker-aware transcription
			return tc.formatSpeakerTranscription(result), nil
		case "ERROR":
			return "", fmt.Errorf("transcription failed: task status is ERROR")
		}

		// Wait before polling again
		time.Sleep(1 * time.Second)
	}
}

func (tc *TranscriberClient) formatSpeakerTranscription(result *DownloadResponse) string {
	var transcription strings.Builder
	speakerMap := make(map[int]string)
	speakerCounter := 1

	// Process each speech result in chronological order
	for _, speechResult := range *result {
		// Skip results with speaker_id = -1 (end of utterance markers)
		if speechResult.SpeakerInfo.SpeakerID == -1 {
			continue
		}

		// Map speaker ID to readable label
		speakerID := speechResult.SpeakerInfo.SpeakerID
		if _, exists := speakerMap[speakerID]; !exists {
			speakerMap[speakerID] = fmt.Sprintf("Speaker %d", speakerCounter)
			speakerCounter++
		}
		speakerLabel := speakerMap[speakerID]

		// Add each text result for this speaker
		for _, textResult := range speechResult.Results {
			if textResult.Text != "" {
				transcription.WriteString(fmt.Sprintf("[%s]: %s\n", speakerLabel, textResult.Text))
			}
		}
	}

	return strings.TrimSpace(transcription.String())
}
