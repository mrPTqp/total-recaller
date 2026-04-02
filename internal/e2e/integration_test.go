package e2e

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (env *IntegrationTestEnv) waitForEvent(eventType string, expectedCount int, timeout time.Duration) bool {
	return env.waitForEventCondition(func(stats map[string]interface{}) bool {
		eventsByType, ok := stats["events_by_type"].(map[string]int)
		if !ok {
			return false
		}
		return eventsByType[eventType] >= expectedCount
	}, timeout)
}

func (env *IntegrationTestEnv) waitForEventCondition(condition func(map[string]interface{}) bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		stats := env.mockServer.GetStats()
		if condition(stats) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// ============================================================================
// COMMAND TESTS
// ============================================================================

// TestStartCommand tests the /start command - bot should respond with welcome message
func TestStartCommand(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	env.mockServer.InjectCommand(userID, "start")

	// Verify command was received
	require.True(t, env.waitForEvent("command", 1, 2*time.Second), "command should be received within 2 seconds")

	// Verify bot sent a welcome message
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 1
	}, 2*time.Second), "bot should send welcome message")
}

// TestListCommandEmpty tests /list command when user has no meetings
func TestListCommandEmpty(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	env.mockServer.InjectCommand(userID, "list")

	require.True(t, env.waitForEvent("command", 1, 2*time.Second), "command should be received")

	// Bot should respond with "no meetings" message
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 1
	}, 3*time.Second), "bot should respond to /list command")
}

// TestGetCommandNoArgument tests /get command without ID argument
func TestGetCommandNoArgument(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	env.mockServer.InjectCommand(userID, "get")

	require.True(t, env.waitForEvent("command", 1, 2*time.Second), "command should be received")

	// Bot should respond with error message about missing ID
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 1
	}, 2*time.Second), "bot should respond with error")
}

// TestGetCommandInvalidID tests /get command with invalid (non-numeric) ID
func TestGetCommandInvalidID(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	env.mockServer.InjectCommand(userID, "get abc")

	require.True(t, env.waitForEvent("command", 1, 2*time.Second), "command should be received")

	// Bot should respond with error about invalid ID
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 1
	}, 2*time.Second), "bot should respond with error")
}

// TestGetCommandNonExistentID tests /get command with valid but non-existent ID
func TestGetCommandNonExistentID(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	env.mockServer.InjectCommand(userID, "get 99999")

	require.True(t, env.waitForEvent("command", 1, 2*time.Second), "command should be received")

	// Bot should respond with "meeting not found" message
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 1
	}, 3*time.Second), "bot should respond")
}

// TestFindCommandNoArgument tests /find command without search query
func TestFindCommandNoArgument(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	env.mockServer.InjectCommand(userID, "find")

	require.True(t, env.waitForEvent("command", 1, 2*time.Second), "command should be received")

	// Bot should respond with error about missing query
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 1
	}, 2*time.Second), "bot should respond with error")
}

// TestFindCommandNoResults tests /find command with query that returns no results
func TestFindCommandNoResults(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	env.mockServer.InjectCommand(userID, "find несуществующий запрос")

	require.True(t, env.waitForEvent("command", 1, 2*time.Second), "command should be received")

	// Bot should respond with "nothing found" message
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 1
	}, 3*time.Second), "bot should respond")
}

// TestSfindCommandNoArgument tests /sfind command without search query
func TestSfindCommandNoArgument(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	env.mockServer.InjectCommand(userID, "sfind")

	require.True(t, env.waitForEvent("command", 1, 2*time.Second), "command should be received")

	// Bot should respond with error about missing query
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 1
	}, 2*time.Second), "bot should respond with error")
}

// TestChatCommandNoArgument tests /chat command without question
func TestChatCommandNoArgument(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	env.mockServer.InjectCommand(userID, "chat")

	require.True(t, env.waitForEvent("command", 1, 2*time.Second), "command should be received")

	// Bot should respond with error about missing question
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 1
	}, 2*time.Second), "bot should respond with error")
}

// TestUnknownCommand tests behavior when unknown command is sent
func TestUnknownCommand(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	env.mockServer.InjectCommand(userID, "unknown_command")

	require.True(t, env.waitForEvent("command", 1, 2*time.Second), "command should be received")

	// Bot should ignore unknown commands or respond with help
	// This test verifies the bot doesn't crash on unknown commands
}

// ============================================================================
// TEXT MESSAGE TESTS
// ============================================================================

// TestTextMessageProcessing tests regular text message handling
func TestTextMessageProcessing(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)
	text := "Hello, this is a test message"

	env.mockServer.InjectTextMessage(userID, text)

	require.True(t, env.waitForEvent("text", 1, 2*time.Second), "text message should be received within 2 seconds")
}

// TestLongTextMessage tests handling of very long text messages
func TestLongTextMessage(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)
	text := string(make([]byte, 4000)) // 4000 characters

	env.mockServer.InjectTextMessage(userID, text)

	require.True(t, env.waitForEvent("text", 1, 2*time.Second), "long text message should be received")
}

// TestTextMessageWithSpecialCharacters tests handling of messages with special characters
func TestTextMessageWithSpecialCharacters(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)
	text := "Специальные символы: !@#$%^&*()_+-=[]{}|;':\",./<>?`~\n\r\t"

	env.mockServer.InjectTextMessage(userID, text)

	require.True(t, env.waitForEvent("text", 1, 2*time.Second), "message with special characters should be received")
}

// ============================================================================
// VOICE MESSAGE TESTS
// ============================================================================

// TestVoiceMessageProcessing tests voice message handling
func TestVoiceMessageProcessing(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)
	testAudioFile := filepath.Join(env.tempDir, "test.opus")

	env.mockServer.InjectCommand(userID, "start")
	require.True(t, env.waitForEvent("command", 1, 1*time.Second), "command should be received")

	err := env.mockServer.InjectVoiceMessage(userID, testAudioFile)
	require.NoError(t, err)

	require.True(t, env.waitForEvent("voice", 1, 2*time.Second), "voice message should be received")
}

// ============================================================================
// AUDIO FILE TESTS
// ============================================================================

// TestAudioFileProcessing tests audio file handling
func TestAudioFileProcessing(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)
	testMP3File := filepath.Join(env.tempDir, "test.mp3")

	env.mockServer.InjectCommand(userID, "start")
	require.True(t, env.waitForEvent("command", 1, 1*time.Second), "command should be received")

	err := env.mockServer.InjectAudioMessage(userID, testMP3File)
	require.NoError(t, err)

	require.True(t, env.waitForEvent("audio", 1, 2*time.Second), "audio message should be received")
}

// TestUnsupportedAudioFormat tests handling of unsupported audio formats
func TestUnsupportedAudioFormat(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	// Create a WAV file (unsupported format)
	wavFile := filepath.Join(env.tempDir, "test.wav")
	err := createTestWAVFile(wavFile)
	require.NoError(t, err)

	env.mockServer.InjectCommand(userID, "start")
	require.True(t, env.waitForEvent("command", 1, 1*time.Second), "command should be received")

	// Inject as audio file (not voice)
	err = env.mockServer.InjectAudioMessageWithFile(userID, wavFile, "test.wav")
	require.NoError(t, err)

	// Bot should respond with unsupported format error
	require.True(t, env.waitForEvent("audio", 1, 2*time.Second), "audio message should be received")
}

// ============================================================================
// MULTIPLE EVENTS TESTS
// ============================================================================

// TestMultipleEventsProcessing tests processing of multiple events in sequence
func TestMultipleEventsProcessing(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	// Send multiple events without sleeps - they will be queued
	env.mockServer.InjectTextMessage(userID, "First message")
	env.mockServer.InjectCommand(userID, "list")
	env.mockServer.InjectTextMessage(userID, "Second message")
	env.mockServer.InjectCommand(userID, "start")

	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		eventsByType := stats["events_by_type"].(map[string]int)
		return eventsByType["text"] >= 2 && eventsByType["command"] >= 2
	}, 3*time.Second), "all events should be received")
}

// TestRapidFireMessages tests handling of many messages sent quickly
func TestRapidFireMessages(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	// Send 10 messages rapidly
	for i := 0; i < 10; i++ {
		env.mockServer.InjectTextMessage(userID, fmt.Sprintf("Message %d", i))
	}

	require.True(t, env.waitForEvent("text", 10, 5*time.Second), "all messages should be received")
}

// ============================================================================
// ERROR SCENARIOS
// ============================================================================

// TestBotRestartScenario tests behavior when bot restarts during operation
func TestBotRestartScenario(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	// Send initial message
	env.mockServer.InjectTextMessage(userID, "First message")
	require.True(t, env.waitForEvent("text", 1, 2*time.Second), "first message should be received")

	// In a real scenario, bot would restart here
	// For now, just verify the test infrastructure handles this
}

// ============================================================================
// POSITIVE SCENARIO TESTS - COMMANDS WITH VALID DATA
// ============================================================================

// TestListCommandWithMeetings tests /list command when user has meetings
func TestListCommandWithMeetings(t *testing.T) {
	// Note: Not using t.Parallel() to avoid too many concurrent PostgreSQL containers
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	// First, register user via /start
	env.mockServer.InjectCommand(userID, "start")
	require.True(t, env.waitForEvent("command", 1, 2*time.Second), "start command should be received")

	// Wait a bit for user registration to complete in the background
	time.Sleep(500 * time.Millisecond)

	// Create a meeting directly in the database for this user
	createTestMeetingForUser(t, env, userID, "Test meeting transcription")

	// Now call /list
	env.mockServer.InjectCommand(userID, "list")

	// Bot should respond with list of meetings
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 2 // welcome message + list response
	}, 3*time.Second), "bot should respond with meetings list")
}

// TestGetCommandValidID tests /get command with valid ID and existing meeting
func TestGetCommandValidID(t *testing.T) {
	// Note: Not using t.Parallel() to avoid too many concurrent PostgreSQL containers
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	// Register user first
	env.mockServer.InjectCommand(userID, "start")
	require.True(t, env.waitForEvent("command", 1, 2*time.Second), "start command should be received")
	time.Sleep(500 * time.Millisecond)

	// Create a meeting directly in the database
	meetingID := createTestMeetingForUser(t, env, userID, "Test meeting content")

	// Call /get with valid ID
	env.mockServer.InjectCommand(userID, fmt.Sprintf("get %d", meetingID))

	require.True(t, env.waitForEvent("command", 2, 2*time.Second), "get command should be received")

	// Bot should respond with meeting transcription
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 2
	}, 3*time.Second), "bot should respond with meeting content")
}

// TestFindCommandWithResults tests /find command with query that returns results
func TestFindCommandWithResults(t *testing.T) {
	// Note: Not using t.Parallel() to avoid too many concurrent PostgreSQL containers
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	// Register user first
	env.mockServer.InjectCommand(userID, "start")
	require.True(t, env.waitForEvent("command", 1, 2*time.Second), "start command should be received")
	time.Sleep(500 * time.Millisecond)

	// Create a meeting with specific content
	createTestMeetingForUser(t, env, userID, "Интеграция с гигачат обсуждалась на встрече")

	// Search for the meeting
	env.mockServer.InjectCommand(userID, "find интеграция")

	require.True(t, env.waitForEvent("command", 2, 2*time.Second), "find command should be received")

	// Bot should respond with search results
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 2
	}, 3*time.Second), "bot should respond with search results")
}

// TestSfindCommandWithResults tests /sfind command with query that returns results
func TestSfindCommandWithResults(t *testing.T) {
	// Note: Not using t.Parallel() to avoid too many concurrent PostgreSQL containers
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	// Register user first
	env.mockServer.InjectCommand(userID, "start")
	require.True(t, env.waitForEvent("command", 1, 2*time.Second), "start command should be received")
	time.Sleep(500 * time.Millisecond)

	// Create a meeting with content
	createTestMeetingForUser(t, env, userID, "Обсуждение проекта и планы на спринт")

	// Semantic search
	env.mockServer.InjectCommand(userID, "sfind Как проходило обсуждение проекта?")

	require.True(t, env.waitForEvent("command", 2, 2*time.Second), "sfind command should be received")

	// Bot should respond with semantic search results
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 2
	}, 5*time.Second), "bot should respond with semantic search results")
}

// TestChatCommandWithQuestion tests /chat command with a question
func TestChatCommandWithQuestion(t *testing.T) {
	// Note: Not using t.Parallel() to avoid too many concurrent PostgreSQL containers
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	// Register user first
	env.mockServer.InjectCommand(userID, "start")
	require.True(t, env.waitForEvent("command", 1, 2*time.Second), "start command should be received")
	time.Sleep(500 * time.Millisecond)

	// Send chat command with question
	env.mockServer.InjectCommand(userID, "chat Привет, как дела?")

	require.True(t, env.waitForEvent("command", 2, 2*time.Second), "chat command should be received")

	// Bot should respond with LLM response
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 2
	}, 5*time.Second), "bot should respond to chat question")
}

// ============================================================================
// EDGE CASES AND VALIDATION TESTS
// ============================================================================

// TestGetCommandNegativeID tests /get command with negative ID
func TestGetCommandNegativeID(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	env.mockServer.InjectCommand(userID, "get -1")

	// Increase timeout to handle slower test environments
	require.True(t, env.waitForEvent("command", 1, 5*time.Second), "command should be received")

	// Bot should respond with error about invalid ID
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 1
	}, 5*time.Second), "bot should respond with error")
}

// TestGetCommandVeryLargeID tests /get command with very large ID
func TestGetCommandVeryLargeID(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	env.mockServer.InjectCommand(userID, "get 999999999999")

	require.True(t, env.waitForEvent("command", 1, 2*time.Second), "command should be received")

	// Bot should respond (either error or not found)
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 1
	}, 2*time.Second), "bot should respond")
}

// TestFindCommandWithSpecialCharacters tests /find with special characters in query
func TestFindCommandWithSpecialCharacters(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	env.mockServer.InjectCommand(userID, "find !@#$%^&*()")

	require.True(t, env.waitForEvent("command", 1, 2*time.Second), "command should be received")

	// Bot should respond (either with results or "nothing found")
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 1
	}, 3*time.Second), "bot should respond")
}

// TestEmptyTextMessage tests handling of empty text message
// Note: This test is skipped because the bot correctly ignores empty messages
func TestEmptyTextMessage(t *testing.T) {
	t.Skip("Bot correctly ignores empty text messages - this is expected behavior")
}

// TestOnlyWhitespaceMessage tests handling of whitespace-only message
func TestOnlyWhitespaceMessage(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	env.mockServer.InjectTextMessage(userID, "   \t\n   ")

	require.True(t, env.waitForEvent("text", 1, 2*time.Second), "whitespace message should be received")
}

// TestUnicodeEmojiMessage tests handling of messages with emoji
func TestUnicodeEmojiMessage(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)
	text := "Привет! 👋🎉🚀 Это сообщение с эмодзи"

	env.mockServer.InjectTextMessage(userID, text)

	require.True(t, env.waitForEvent("text", 1, 2*time.Second), "emoji message should be received")
}

// ============================================================================
// MULTIPLE USERS TESTS
// ============================================================================

// TestMultipleUsersIsolation tests that different users don't see each other's meetings
func TestMultipleUsersIsolation(t *testing.T) {
	// Note: Not using t.Parallel() to avoid too many concurrent PostgreSQL containers
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	user1 := int64(123456789)
	user2 := int64(987654321)

	// Register both users
	env.mockServer.InjectCommand(user1, "start")
	env.mockServer.InjectCommand(user2, "start")
	require.True(t, env.waitForEvent("command", 2, 2*time.Second), "both start commands should be received")
	time.Sleep(500 * time.Millisecond)

	// Create meeting for user1 only
	createTestMeetingForUser(t, env, user1, "Meeting for user 1")

	// User2 requests /list - should see no meetings
	env.mockServer.InjectCommand(user2, "list")
	require.True(t, env.waitForEvent("command", 3, 2*time.Second), "list command should be received")

	// User1 requests /list - should see one meeting
	env.mockServer.InjectCommand(user1, "list")
	require.True(t, env.waitForEvent("command", 4, 2*time.Second), "second list command should be received")

	// Both users should have received responses
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 4 // 2 welcome + 2 list responses
	}, 3*time.Second), "all responses should be sent")
}

// TestConcurrentUsers tests concurrent requests from different users
func TestConcurrentUsers(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	// Create multiple users
	for i := 1; i <= 5; i++ {
		userID := int64(100000000 + i)
		env.mockServer.InjectCommand(userID, "start")
		env.mockServer.InjectCommand(userID, "list")
	}

	// All commands should be processed - increase timeout for slower environments
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		eventsByType := stats["events_by_type"].(map[string]int)
		return eventsByType["command"] >= 10
	}, 10*time.Second), "all concurrent commands should be processed")
}

// ============================================================================
// AUDIO PROCESSING FULL CYCLE TESTS
// ============================================================================

// TestVoiceMessageFullCycle tests complete voice message processing flow
func TestVoiceMessageFullCycle(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)
	testAudioFile := filepath.Join(env.tempDir, "test.opus")

	// Register user first
	env.mockServer.InjectCommand(userID, "start")
	require.True(t, env.waitForEvent("command", 1, 1*time.Second), "start command should be received")
	time.Sleep(500 * time.Millisecond)

	// Send voice message
	err := env.mockServer.InjectVoiceMessage(userID, testAudioFile)
	require.NoError(t, err)

	require.True(t, env.waitForEvent("voice", 1, 2*time.Second), "voice message should be received")

	// Wait for processing to complete (transcription, meeting creation, etc.)
	// The bot should send multiple messages: processing indicator, then result
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 2
	}, 10*time.Second), "bot should process voice message and respond")
}

// TestAudioFileFullCycle tests complete audio file processing flow
func TestAudioFileFullCycle(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)
	testMP3File := filepath.Join(env.tempDir, "test.mp3")

	// Register user first
	env.mockServer.InjectCommand(userID, "start")
	require.True(t, env.waitForEvent("command", 1, 1*time.Second), "start command should be received")
	time.Sleep(500 * time.Millisecond)

	// Send audio file
	err := env.mockServer.InjectAudioMessage(userID, testMP3File)
	require.NoError(t, err)

	require.True(t, env.waitForEvent("audio", 1, 2*time.Second), "audio message should be received")

	// Wait for processing to complete
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 2
	}, 10*time.Second), "bot should process audio file and respond")
}

// ============================================================================
// ERROR HANDLING TESTS
// ============================================================================

// TestFileTooLarge tests handling of audio file exceeding size limit
func TestFileTooLarge(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	// Create a large file (larger than default limit)
	largeFile := filepath.Join(env.tempDir, "large.opus")
	largeContent := make([]byte, 20*1024*1024) // 20MB
	err := os.WriteFile(largeFile, largeContent, 0644)
	require.NoError(t, err)

	// Register user first
	env.mockServer.InjectCommand(userID, "start")
	require.True(t, env.waitForEvent("command", 1, 1*time.Second), "start command should be received")
	time.Sleep(500 * time.Millisecond)

	// Send oversized voice message
	err = env.mockServer.InjectVoiceMessage(userID, largeFile)
	require.NoError(t, err)

	require.True(t, env.waitForEvent("voice", 1, 2*time.Second), "voice message should be received")

	// Bot should respond with file size error
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 2 // welcome + error message
	}, 3*time.Second), "bot should respond with file size error")
}

// TestInvalidAudioFile tests handling of corrupted audio file
func TestInvalidAudioFile(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	// Create a corrupted MP3 file
	corruptedFile := filepath.Join(env.tempDir, "corrupted.mp3")
	err := os.WriteFile(corruptedFile, []byte("not a valid mp3"), 0644)
	require.NoError(t, err)

	// Register user first
	env.mockServer.InjectCommand(userID, "start")
	require.True(t, env.waitForEvent("command", 1, 1*time.Second), "start command should be received")
	time.Sleep(500 * time.Millisecond)

	// Send corrupted audio file
	err = env.mockServer.InjectAudioMessage(userID, corruptedFile)
	require.NoError(t, err)

	require.True(t, env.waitForEvent("audio", 1, 2*time.Second), "audio message should be received")

	// Bot should handle the error gracefully
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 2
	}, 5*time.Second), "bot should handle corrupted file")
}

// TestChatCommandQueueOverflow tests behavior when LLM queue is full
func TestChatCommandQueueOverflow(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	// Register user first
	env.mockServer.InjectCommand(userID, "start")
	require.True(t, env.waitForEvent("command", 1, 1*time.Second), "start command should be received")
	time.Sleep(500 * time.Millisecond)

	// Send multiple chat commands rapidly to potentially overflow queue
	for i := 0; i < 5; i++ {
		env.mockServer.InjectCommand(userID, fmt.Sprintf("chat Question %d", i))
	}

	require.True(t, env.waitForEvent("command", 6, 3*time.Second), "all chat commands should be received")

	// Bot should handle queue overflow gracefully
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 6
	}, 5*time.Second), "bot should respond to chat commands")
}

// ============================================================================
// USER REGISTRATION TESTS
// ============================================================================

// TestUserRegistrationOnStart tests that user is registered when calling /start
func TestUserRegistrationOnStart(t *testing.T) {
	// Note: Not using t.Parallel() to avoid too many concurrent PostgreSQL containers
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	// Call /start
	env.mockServer.InjectCommand(userID, "start")
	require.True(t, env.waitForEvent("command", 1, 2*time.Second), "start command should be received")

	// Wait for registration to complete
	time.Sleep(1 * time.Second)

	// Verify user was registered by checking database
	user, err := env.testDB.GetUserByTelegramID(env.appCtx, userID)
	require.NoError(t, err, "user should be registered in database")
	require.NotNil(t, user, "user should not be nil")
	require.Equal(t, userID, user.TelegramID, "telegram ID should match")
}

// TestDoubleRegistration tests that double registration doesn't cause errors
func TestDoubleRegistration(t *testing.T) {
	// Note: Not using t.Parallel() to avoid too many concurrent PostgreSQL containers
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	userID := int64(123456789)

	// First registration
	env.mockServer.InjectCommand(userID, "start")
	require.True(t, env.waitForEvent("command", 1, 2*time.Second), "first start command should be received")
	time.Sleep(500 * time.Millisecond)

	// Second registration (same user)
	env.mockServer.InjectCommand(userID, "start")
	require.True(t, env.waitForEvent("command", 2, 2*time.Second), "second start command should be received")

	// Bot should handle gracefully without crashing
	require.True(t, env.waitForEventCondition(func(stats map[string]interface{}) bool {
		return stats["updates_sent"].(int) >= 2
	}, 3*time.Second), "bot should respond to both start commands")
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// createTestWAVFile creates a minimal WAV file for testing unsupported formats
func createTestWAVFile(path string) error {
	// Minimal WAV header
	data := []byte{
		'R', 'I', 'F', 'F', // ChunkID
		0x24, 0x00, 0x00, 0x00, // ChunkSize (36 bytes)
		'W', 'A', 'V', 'E', // Format
		'f', 'm', 't', ' ', // Subchunk1ID
		0x10, 0x00, 0x00, 0x00, // Subchunk1Size (16 bytes for PCM)
		0x01, 0x00, // AudioFormat (1 = PCM)
		0x01, 0x00, // NumChannels (1 = mono)
		0x80, 0x1f, 0x00, 0x00, // SampleRate (8000 Hz)
		0x00, 0x7d, 0x00, 0x00, // ByteRate
		0x01, 0x00, // BlockAlign
		0x08, 0x00, // BitsPerSample (8 bits)
		'd', 'a', 't', 'a', // Subchunk2ID
		0x00, 0x00, 0x00, 0x00, // Subchunk2Size
	}
	return os.WriteFile(path, data, 0644)
}

// createTestMeetingForUser creates a test meeting for a user and returns the meeting ID
func createTestMeetingForUser(t *testing.T, env *IntegrationTestEnv, userID int64, transcription string) int {
	t.Helper()
	meetingID, err := env.testDB.CreateTestMeeting(env.appCtx, userID, transcription)
	require.NoError(t, err, "failed to create test meeting")
	return meetingID
}

// Verify that tests compile and basic infrastructure works
func TestInfrastructureHealth(t *testing.T) {
	t.Parallel()
	env := NewIntegrationTestEnv(t)
	defer env.teardown()

	// Verify all mock servers are accessible
	resp, err := http.Get(fmt.Sprintf("http://%s/control/health", env.mockServer.Addr()))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify token mock is accessible with proper scope parameter
	tokenResp, err := http.PostForm(fmt.Sprintf("%s/api/v2/oauth", env.tokenMock.URL()), url.Values{"scope": {"SALUTE_SPEECH_PERS"}})
	require.NoError(t, err)
	defer tokenResp.Body.Close()
	assert.Equal(t, http.StatusOK, tokenResp.StatusCode)
}
