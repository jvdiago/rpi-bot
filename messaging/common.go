package messaging

import "context"

// LLMType is an enum representing the supported LLM providers.
type MessageType int

const (
	Command MessageType = iota
	Response
	Chat
	Update
)

type Message struct {
	Type    MessageType
	Command string
	Args    []string
	Raw     string
	Text    string
	ChatID  int64  //For telegram
	Source  string //For Signal
}

type MessageReceiver interface {
	GetUpdates(ctx context.Context) <-chan Message
}

type MessageSender interface {
	SendMessage(message string, replyTo Message) error
}
type MessageClient interface {
	MessageReceiver
	MessageSender
}
