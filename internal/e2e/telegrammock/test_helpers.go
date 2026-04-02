package telegrammock

import (
	"os"
	"strings"
)

// CreateTestAudioFile creates a test audio file with minimal Opus header.
func CreateTestAudioFile(path string) error {
	content := []byte("OpusHead" + strings.Repeat("\x00", 50))
	return os.WriteFile(path, content, 0644)
}

// CreateTestMP3File creates a test MP3 file with minimal ID3 header.
func CreateTestMP3File(path string) error {
	content := []byte("ID3" + strings.Repeat("\x00", 50))
	return os.WriteFile(path, content, 0644)
}

// CreateTestDir creates a temporary directory for test files.
func CreateTestDir() (string, error) {
	return os.MkdirTemp("", "telegram-mock-test")
}