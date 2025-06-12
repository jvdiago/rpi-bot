package messaging

import (
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/require"
)

// makeCommandMessage constructs a tgbotapi.Update that Telegram considers a command.
// It sets Text beginning with "/" and includes a MessageEntity of type "bot_command"
// at offset 0 with the correct length.
func makeCommandMessage(text string, chatID int64) *tgbotapi.Update {
	// Determine the length of the command part (up to first space or entire text).
	var cmdLen int
	for i, ch := range text {
		if ch == ' ' {
			cmdLen = i
			break
		}
	}
	if cmdLen == 0 {
		cmdLen = len(text)
	}

	return &tgbotapi.Update{
		Message: &tgbotapi.Message{
			Text: text,
			Chat: &tgbotapi.Chat{ID: chatID},
			Entities: []tgbotapi.MessageEntity{
				{
					Type:   "bot_command",
					Offset: 0,
					Length: cmdLen,
				},
			},
		},
	}
}

func TestParseTelegramMessage(t *testing.T) {
	tests := []struct {
		name        string
		update      *tgbotapi.Update
		wantType    MessageType
		wantCommand string
		wantArgs    []string
		wantRaw     string
		wantChatID  int64
	}{
		{
			name:        "command with arguments",
			update:      makeCommandMessage("/greet Alice Bob", 12345),
			wantType:    Command,
			wantCommand: "greet",
			wantArgs:    []string{"Alice", "Bob"},
			wantRaw:     "/greet Alice Bob",
			wantChatID:  12345,
		},
		{
			name: "regular chat message",
			update: &tgbotapi.Update{
				Message: &tgbotapi.Message{
					Text: "hello world",
					Chat: &tgbotapi.Chat{ID: 98765},
				},
			},
			wantType:    Chat,
			wantCommand: "",
			wantArgs:    []string(nil),
			wantRaw:     "hello world",
			wantChatID:  98765,
		},
		{
			name:        "update with nil Message",
			update:      &tgbotapi.Update{Message: nil},
			wantType:    Update,
			wantCommand: "",
			wantArgs:    []string(nil),
			wantRaw:     "",
			wantChatID:  0,
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			msg := parseTelegramMessage(tt.update)

			require.Equal(t, tt.wantType, msg.Type, "Type mismatch")
			require.Equal(t, tt.wantCommand, msg.Command, "Command mismatch")
			require.Equal(t, tt.wantArgs, msg.Args, "Args mismatch")
			require.Equal(t, tt.wantRaw, msg.Raw, "Raw mismatch")
			require.Equal(t, tt.wantChatID, msg.ChatID, "ChatID mismatch")
		})
	}
}
