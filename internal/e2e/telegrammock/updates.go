package telegrammock

import (
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	tele "gopkg.in/telebot.v3"
)

// Update is a type alias for tele.Update to simplify imports in other files.
type Update = tele.Update

func (m *TelegramAPIMock) createTextMessageUpdate(userID int64, text string) Update {
	return Update{
		ID: int(m.generateUpdateID()),
		Message: &tele.Message{
			ID:       int(m.generateUpdateID()),
			Sender:   &tele.User{ID: userID, FirstName: "Test", LastName: "User"},
			Chat:     &tele.Chat{ID: userID, Type: "private"},
			Unixtime: time.Now().Unix(),
			Text:     text,
		},
	}
}

func (m *TelegramAPIMock) createVoiceMessageUpdate(userID int64, filePath string) (Update, error) {
	fileID := uuid.New().String()

	// Store file mapping
	m.filesMu.Lock()
	m.fileStorage[fileID] = filePath
	m.filesMu.Unlock()

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return Update{}, err
	}

	return Update{
		ID: int(m.generateUpdateID()),
		Message: &tele.Message{
			ID:       int(m.generateUpdateID()),
			Sender:   &tele.User{ID: userID, FirstName: "Test", LastName: "User"},
			Chat:     &tele.Chat{ID: userID, Type: "private"},
			Unixtime: time.Now().Unix(),
			Voice: &tele.Voice{
				File:     tele.File{FileID: fileID, FilePath: filePath, FileSize: fileInfo.Size()},
				Duration: 0,
			},
		},
	}, nil
}

func (m *TelegramAPIMock) createAudioMessageUpdate(userID int64, filePath string) (Update, error) {
	return m.createAudioMessageUpdateWithFileName(userID, filePath, filepath.Base(filePath))
}

func (m *TelegramAPIMock) createAudioMessageUpdateWithFileName(userID int64, filePath string, fileName string) (Update, error) {
	fileID := uuid.New().String()

	// Store file mapping
	m.filesMu.Lock()
	m.fileStorage[fileID] = filePath
	m.filesMu.Unlock()

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return Update{}, err
	}

	return Update{
		ID: int(m.generateUpdateID()),
		Message: &tele.Message{
			ID:       int(m.generateUpdateID()),
			Sender:   &tele.User{ID: userID, FirstName: "Test", LastName: "User"},
			Chat:     &tele.Chat{ID: userID, Type: "private"},
			Unixtime: time.Now().Unix(),
			Audio: &tele.Audio{
				File:     tele.File{FileID: fileID, FilePath: filePath, FileSize: fileInfo.Size()},
				FileName: fileName,
				Duration: 0,
			},
		},
	}, nil
}

func (m *TelegramAPIMock) createCommandUpdate(userID int64, command string) Update {
	text := "/" + command

	entities := []tele.MessageEntity{
		{
			Type:   tele.EntityCommand,
			Offset: 0,
			Length: len(text),
		},
	}

	return Update{
		ID: int(m.generateUpdateID()),
		Message: &tele.Message{
			ID:       int(m.generateUpdateID()),
			Sender:   &tele.User{ID: userID, FirstName: "Test", LastName: "User"},
			Chat:     &tele.Chat{ID: userID, Type: "private"},
			Unixtime: time.Now().Unix(),
			Text:     text,
			Entities: entities,
		},
	}
}