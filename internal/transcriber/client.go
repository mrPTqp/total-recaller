package transcriber

import (
	"bytes"
	"context"
	"encoding/json"
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
	httpClient   *http.Client
	tokenManager *token.TokenManager
	logger       *zap.Logger
}

type UploadResponse struct {
	Result struct {
		RequestFileID string `json:"request_file_id"`
	} `json:"result"`
}

type RecognizeRequest struct {
	Options struct {
		Model         string `json:"model"`
		AudioEncoding string `json:"audio_encoding"`
		SampleRate    int    `json:"sample_rate"`
		ChannelsCount int    `json:"channels_count"`
	} `json:"options"`
	RequestFileID string `json:"request_file_id"`
}

type RecognizeResponse struct {
	Result struct {
		ID string `json:"id"`
	} `json:"result"`
}

type StatusResponse struct {
	Result struct {
		Status         string `json:"status"`
		ResponseFileID string `json:"response_file_id"`
	} `json:"result"`
}

type DownloadResponse []SaluteSpeechResult

type SaluteSpeechResult struct {
	Results             []SaluteTextResult `json:"results"`
	Eou                 bool               `json:"eou"`
	EmotionsResult      EmotionsResult     `json:"emotions_result"`
	ProcessedAudioStart Timestamp          `json:"processed_audio_start"`
	ProcessedAudioEnd   Timestamp          `json:"processed_audio_end"`
	BackendInfo         BackendInfo        `json:"backend_info"`
	Channel             int                `json:"channel"`
	SpeakerInfo         SpeakerInfo        `json:"speaker_info"`
	EouReason           string             `json:"eou_reason"`
	Insight             string             `json:"insight"`
	PersonIdentity      PersonIdentity     `json:"person_identity"`
}

type SaluteTextResult struct {
	Text           string                `json:"text"`
	NormalizedText string                `json:"normalized_text"`
	Start          Timestamp             `json:"start"`
	End            Timestamp             `json:"end"`
	WordAlignments []SaluteWordAlignment `json:"word_alignments"`
}

type SaluteWordAlignment struct {
	Word  string    `json:"word"`
	Start Timestamp `json:"start"`
	End   Timestamp `json:"end"`
}

type EmotionsResult struct {
	Positive float64 `json:"positive"`
	Negative float64 `json:"negative"`
	Neutral  float64 `json:"neutral"`
}

type BackendInfo struct {
	ModelName     string `json:"model_name"`
	ModelVersion  string `json:"model_version"`
	ServerVersion string `json:"server_version"`
}

type SpeakerInfo struct {
	SpeakerID             int     `json:"speaker_id"`
	MainSpeakerConfidence float64 `json:"main_speaker_confidence"`
}

type PersonIdentity struct {
	Age         string  `json:"age"`
	Gender      string  `json:"gender"`
	AgeScore    float64 `json:"age_score"`
	GenderScore float64 `json:"gender_score"`
}

type Timestamp time.Duration

func (t *Timestamp) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	s = strings.Trim(s, `"`)

	// Parse the duration (format: "0.640s")
	d, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid timestamp format %q: %w", s, err)
	}

	*t = Timestamp(d)
	return nil
}

func (t Timestamp) MarshalJSON() ([]byte, error) {
	d := time.Duration(t)
	return json.Marshal(d.String())
}

func (t Timestamp) Duration() time.Duration {
	return time.Duration(t)
}

func (t Timestamp) Seconds() float64 {
	return t.Duration().Seconds()
}

func (t Timestamp) String() string {
	return t.Duration().String()
}

func NewTranscriberClient(config *config.Config, httpClient *http.Client, tokenManager *token.TokenManager, logger *zap.Logger) *TranscriberClient {
	return &TranscriberClient{
		config:       config,
		httpClient:   httpClient,
		tokenManager: tokenManager,
		logger:       logger,
	}
}

func (tc *TranscriberClient) UploadFile(ctx context.Context, audio io.ReadCloser) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", tc.config.Transcriber.SaluteURL+"/data:upload", audio)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+tc.tokenManager.GetToken(tc.config.Transcriber.TokenManager.Scope))

	resp, err := tc.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(responseBody))
	}

	var uploadResp UploadResponse
	err = json.NewDecoder(resp.Body).Decode(&uploadResp)
	if err != nil {
		return "", fmt.Errorf("failed to decode upload response: %w", err)
	}

	return uploadResp.Result.RequestFileID, nil
}

func (tc *TranscriberClient) CreateRecognitionTask(ctx context.Context, requestFileID string, audioEncoding string) (string, error) {
	recognizeReq := RecognizeRequest{}
	recognizeReq.Options.Model = "general"
	recognizeReq.Options.AudioEncoding = audioEncoding
	recognizeReq.Options.SampleRate = 16000
	recognizeReq.RequestFileID = requestFileID
	recognizeReq.Options.ChannelsCount = 1

	body, err := json.Marshal(recognizeReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal recognize request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", tc.config.Transcriber.SaluteURL+"/speech:async_recognize", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create recognize request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tc.tokenManager.GetToken(tc.config.Transcriber.TokenManager.Scope))

	resp, err := tc.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create recognition task: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("recognition task creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	var recognizeResp RecognizeResponse
	err = json.NewDecoder(resp.Body).Decode(&recognizeResp)
	if err != nil {
		return "", fmt.Errorf("failed to decode recognize response: %w", err)
	}

	return recognizeResp.Result.ID, nil
}

func (tc *TranscriberClient) CheckTaskStatus(ctx context.Context, taskID string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/task:get?id=%s", tc.config.Transcriber.SaluteURL, taskID), nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create status request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+tc.tokenManager.GetToken(tc.config.Transcriber.TokenManager.Scope))

	resp, err := tc.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to check task status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("status check failed with status %d: %s", resp.StatusCode, string(body))
	}

	var statusResp StatusResponse
	err = json.NewDecoder(resp.Body).Decode(&statusResp)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode status response: %w", err)
	}

	return statusResp.Result.Status, statusResp.Result.ResponseFileID, nil
}

func (tc *TranscriberClient) DownloadResult(ctx context.Context, responseFileID string) (*DownloadResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/data:download?response_file_id=%s", tc.config.Transcriber.SaluteURL, responseFileID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+tc.tokenManager.GetToken(tc.config.Transcriber.TokenManager.Scope))

	resp, err := tc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download result: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for logging
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d: %s", resp.StatusCode, string(responseBody))
	}

	// Log the response body
	fmt.Printf("Download response body: %s\n", string(responseBody))

	var downloadResp DownloadResponse
	err = json.NewDecoder(bytes.NewReader(responseBody)).Decode(&downloadResp)
	if err != nil {
		return nil, fmt.Errorf("failed to decode download response: %w", err)
	}

	return &downloadResp, nil
}

func (tc *TranscriberClient) TranscribeFile(ctx context.Context, audio io.ReadCloser, audioEncoding string) (string, error) {
	// Step 1: Upload file
	requestFileID, err := tc.UploadFile(ctx, audio)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	// Step 2: Create recognition task
	taskID, err := tc.CreateRecognitionTask(ctx, requestFileID, audioEncoding)
	if err != nil {
		return "", fmt.Errorf("failed to create recognition task: %w", err)
	}

	// Step 3: Poll for completion
	for {
		status, responseFileID, err := tc.CheckTaskStatus(ctx, taskID)
		if err != nil {
			return "", fmt.Errorf("failed to check task status: %w", err)
		}

		switch status {
		case "DONE":
			// Step 4: Download result
			result, err := tc.DownloadResult(ctx, responseFileID)
			if err != nil {
				return "", fmt.Errorf("failed to download result: %w", err)
			}

			// Extract text from result
			var transcription strings.Builder
			for _, speechResult := range *result {
				for _, textResult := range speechResult.Results {
					transcription.WriteString(textResult.Text + " ")
				}
			}
			return strings.TrimSpace(transcription.String()), nil
		case "ERROR":
			return "", fmt.Errorf("transcription failed: task status is ERROR")
		}

		// Wait before polling again
		time.Sleep(1 * time.Second)
	}
}
