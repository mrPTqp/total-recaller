package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mrPTqp/total-recaller/internal/integration/llmmock"
	"github.com/mrPTqp/total-recaller/internal/integration/telegrammock"
	"github.com/mrPTqp/total-recaller/internal/integration/tokenmock"
	"github.com/mrPTqp/total-recaller/internal/integration/transcribermock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type IntegrationTestEnv struct {
	t               *testing.T
	mockServer      *telegrammock.TelegramAPIMock
	transcriberMock *transcribermock.TranscriberMock
	llmMock         *llmmock.LLMMock
	tokenMock       *tokenmock.TokenMock
	testDB          *TestDB
	tempDir         string
	logger          *zap.Logger
	appCmd          *exec.Cmd
	appCtx          context.Context
	appCancel       context.CancelFunc
	configPath      string
}

func NewIntegrationTestEnv(t *testing.T) *IntegrationTestEnv {
	env := &IntegrationTestEnv{
		t: t,
	}

	env.setup()
	return env
}

func (env *IntegrationTestEnv) setup() {
	var err error

	env.tempDir, err = os.MkdirTemp("", "integration-test")
	require.NoError(env.t, err)

	testAudioFile := filepath.Join(env.tempDir, "test.opus")
	testMP3File := filepath.Join(env.tempDir, "test.mp3")

	err = telegrammock.CreateTestAudioFile(testAudioFile)
	require.NoError(env.t, err)

	err = telegrammock.CreateTestMP3File(testMP3File)
	require.NoError(env.t, err)

	env.logger, err = zap.NewDevelopment(zap.WithCaller(true))
	require.NoError(env.t, err)

	env.appCtx, env.appCancel = context.WithCancel(context.Background())

	// Navigate to project root from internal/integration/
	// Go up 2 levels: internal/integration -> internal -> project root
	wd, err := os.Getwd()
	require.NoError(env.t, err)
	projectRoot := filepath.Join(wd, "..", "..")
	migrationsPath := filepath.Join(projectRoot, "migrations")

	env.testDB, err = NewTestDB(env.appCtx, env.t, migrationsPath)
	require.NoError(env.t, err)

	env.mockServer, err = telegrammock.NewTelegramAPIMock("test-token", ":0", env.logger)
	require.NoError(env.t, err)

	err = env.mockServer.StartAsync()
	require.NoError(env.t, err)

	// Create centralized token mock for both transcriber and LLM
	env.tokenMock, err = tokenmock.NewTokenMock(":0", env.logger)
	require.NoError(env.t, err)

	err = env.tokenMock.StartAsync()
	require.NoError(env.t, err)

	// Create transcriber mock (API only, no token server)
	env.transcriberMock, err = transcribermock.NewTranscriberMock(":0", env.logger)
	require.NoError(env.t, err)

	err = env.transcriberMock.StartAsync()
	require.NoError(env.t, err)

	// Create LLM mock (API only, no token server)
	env.llmMock, err = llmmock.NewLLMMock(":0", env.logger)
	require.NoError(env.t, err)

	err = env.llmMock.StartAsync()
	require.NoError(env.t, err)

	env.configPath = filepath.Join(env.tempDir, "config.yaml")

	configPath := filepath.Join(wd, "config_test.yaml")
	testConfigContent, err := os.ReadFile(configPath)
	require.NoError(env.t, err)

	err = os.WriteFile(env.configPath, testConfigContent, 0644)
	require.NoError(env.t, err)

	err = env.startApp()
	require.NoError(env.t, err)

	env.waitForAppReady(2 * time.Second)
}

// startApp starts the application as a separate process.
func (env *IntegrationTestEnv) startApp() error {
	appEnv := os.Environ()
	appEnv = append(appEnv,
		// Dynamic values (cannot be in config file)
		fmt.Sprintf("TOTAL_RECALLER_BOT_TELEGRAM_API_URL=%s", env.mockServer.BaseURL()),
		fmt.Sprintf("TOTAL_RECALLER_DATABASE_DSN=%s", env.testDB.ConnStr),
		fmt.Sprintf("TOTAL_RECALLER_TRANSCRIBER_TOKEN_MANAGER_URL=%s", env.tokenMock.URL()),
		fmt.Sprintf("TOTAL_RECALLER_TRANSCRIBER_SALUTE_URL=%s", env.transcriberMock.APIURL()),
		fmt.Sprintf("TOTAL_RECALLER_LLM_TOKEN_MANAGER_URL=%s", env.tokenMock.URL()),
		// Sensitive data (test credentials)
		fmt.Sprintf("TOTAL_RECALLER_BOT_TOKEN=%s", "test-token"),
		"TOTAL_RECALLER_TRANSCRIBER_TOKEN_MANAGER_CREDENTIALS=test-credentials",
		"TOTAL_RECALLER_LLM_TOKEN_MANAGER_CREDENTIALS=test-credentials",
	)

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Navigate to project root from internal/integration/
	// Go up 2 levels: internal/integration -> internal -> project root
	projectRoot := filepath.Join(wd, "..", "..")

	env.appCmd = exec.CommandContext(env.appCtx, "go", "run", "cmd/bot/main.go")
	env.appCmd.Dir = projectRoot
	env.appCmd.Env = appEnv
	// Don't inherit stdout/stderr to avoid I/O issues on cleanup
	env.appCmd.Stdout = nil
	env.appCmd.Stderr = nil

	err = env.appCmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start app: %w", err)
	}

	env.logger.Info("Application started", zap.Int("pid", env.appCmd.Process.Pid))

	return nil
}

func (env *IntegrationTestEnv) waitForAppReady(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		stats := env.mockServer.GetStats()
		// App is ready when it has made at least one getUpdates request
		// (which means it's connected and polling)
		if pollingRequests, ok := stats["polling_requests"].(int); ok && pollingRequests > 0 {
			// Give the app a moment to complete its first polling cycle
			time.Sleep(100 * time.Millisecond)
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func (env *IntegrationTestEnv) teardown() {
	// Cancel the app context to stop the process
	if env.appCancel != nil {
		env.appCancel()
	}

	// Give the process a moment to clean up before killing
	time.Sleep(500 * time.Millisecond)

	// Kill the process if it's still running
	if env.appCmd != nil && env.appCmd.Process != nil {
		env.logger.Info("Killing application process", zap.Int("pid", env.appCmd.Process.Pid))
		env.appCmd.Process.Kill()
		// Wait for process to exit
		env.appCmd.Wait()
	}

	if env.mockServer != nil {
		env.mockServer.Stop()
	}

	if env.transcriberMock != nil {
		env.transcriberMock.Stop()
	}

	if env.llmMock != nil {
		env.llmMock.Stop()
	}

	if env.tokenMock != nil {
		env.tokenMock.Stop()
	}

	if env.testDB != nil {
		env.testDB.Close(env.appCtx)
	}

	if env.tempDir != "" {
		os.RemoveAll(env.tempDir)
	}
}

// GetTranscriberMock returns the transcriber mock server for test assertions
func (env *IntegrationTestEnv) GetTranscriberMock() *transcribermock.TranscriberMock {
	return env.transcriberMock
}

// GetLLMMock returns the LLM mock server for test assertions
func (env *IntegrationTestEnv) GetLLMMock() *llmmock.LLMMock {
	return env.llmMock
}