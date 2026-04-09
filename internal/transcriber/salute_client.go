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

type SaluteClient struct {
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
		Model                    string                   `json:"model"`
		AudioEncoding            string                   `json:"audio_encoding"`
		SampleRate               int                      `json:"sample_rate"`
		ChannelsCount            int                      `json:"channels_count"`
		SpeakerSeparationOptions SpeakerSeparationOptions `json:"speaker_separation_options"`
	} `json:"options"`
	RequestFileID string `json:"request_file_id"`
}

type SpeakerSeparationOptions struct {
	Enable                bool `json:"enable"`
	EnableOnlyMainSpeaker bool `json:"enable_only_main_speaker"`
	Count                 int  `json:"count"`
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

func NewSaluteClient(config *config.Config, httpClient *http.Client, tokenManager *token.TokenManager, logger *zap.Logger) *SaluteClient {
	return &SaluteClient{
		config:       config,
		httpClient:   httpClient,
		tokenManager: tokenManager,
		logger:       logger,
	}
}

func (sc *SaluteClient) UploadFile(ctx context.Context, audio io.ReadCloser) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", sc.config.Transcriber.SaluteURL+"/data:upload", audio)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+sc.tokenManager.GetToken(sc.config.Transcriber.TokenManager.Scope))

	resp, err := sc.httpClient.Do(req)
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

func (sc *SaluteClient) CreateRecognitionTask(ctx context.Context, requestFileID string, audioEncoding string) (string, error) {
	recognizeReq := RecognizeRequest{}
	recognizeReq.Options.Model = "general"
	recognizeReq.Options.AudioEncoding = audioEncoding
	recognizeReq.Options.SampleRate = 16000
	recognizeReq.RequestFileID = requestFileID
	recognizeReq.Options.ChannelsCount = 1
	recognizeReq.Options.SpeakerSeparationOptions.Enable = true
	recognizeReq.Options.SpeakerSeparationOptions.EnableOnlyMainSpeaker = false
	recognizeReq.Options.SpeakerSeparationOptions.Count = 10

	body, err := json.Marshal(recognizeReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal recognize request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", sc.config.Transcriber.SaluteURL+"/speech:async_recognize", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create recognize request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+sc.tokenManager.GetToken(sc.config.Transcriber.TokenManager.Scope))

	resp, err := sc.httpClient.Do(req)
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

func (sc *SaluteClient) CheckTaskStatus(ctx context.Context, taskID string) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/task:get?id=%s", sc.config.Transcriber.SaluteURL, taskID), nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create status request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+sc.tokenManager.GetToken(sc.config.Transcriber.TokenManager.Scope))

	resp, err := sc.httpClient.Do(req)
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

func (sc *SaluteClient) DownloadResult(ctx context.Context, responseFileID string) (*DownloadResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/data:download?response_file_id=%s", sc.config.Transcriber.SaluteURL, responseFileID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+sc.tokenManager.GetToken(sc.config.Transcriber.TokenManager.Scope))

	resp, err := sc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download result: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d: %s", resp.StatusCode, string(responseBody))
	}

	fmt.Printf("Download response body: %s\n", string(responseBody))

	var downloadResp DownloadResponse
	err = json.NewDecoder(bytes.NewReader(responseBody)).Decode(&downloadResp)
	if err != nil {
		return nil, fmt.Errorf("failed to decode download response: %w", err)
	}

	return &downloadResp, nil
}