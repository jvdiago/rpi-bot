package messaging

import (
	"context"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type telegramReceiver struct {
	apitoken string
	ch       chan Message
	debug    bool
	bot      *tgbotapi.BotAPI
}

func NewTelegramReceiver(apitoken string, debug bool) (*telegramReceiver, error) {
	bot, err := tgbotapi.NewBotAPI(apitoken)
	if err != nil {
		return nil, err
	}

	bot.Debug = debug

	r := &telegramReceiver{
		ch:       make(chan Message, 10),
		apitoken: apitoken,
		debug:    debug,
		bot:      bot,
	}
	return r, nil
}

func (s *telegramReceiver) GetUpdates(ctx context.Context) <-chan Message {
	go s.messageReceiver(ctx)
	return s.ch
}

func parseTelegramMessage(update *tgbotapi.Update) Message {
	if update.Message == nil {
		return Message{
			Type: Update,
		}
	}

	messageType := Command
	if !update.Message.IsCommand() {
		messageType = Chat
	}
	var args []string

	if update.Message.CommandArguments() != "" {
		args = strings.Split(update.Message.CommandArguments(), " ")
	}

	m := Message{
		Type:    messageType,
		Raw:     update.Message.Text,
		Command: update.Message.Command(),
		ChatID:  update.Message.Chat.ID,
		Args:    args,
	}
	return m
}

func (t *telegramReceiver) messageReceiver(ctx context.Context) {
	defer close(t.ch)
	log.Printf("Authorized on account %s", t.bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 5

	updates := t.bot.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			// If our context is cancelled, bail out immediately.
			t.bot.StopReceivingUpdates()
			log.Println("telegramReceiver: context done, returning")
			return

		case update, ok := <-updates:
			if !ok {
				// The updates channel has been closed by the library.
				log.Println("telegramReceiver: updates channel closed, exiting")
				return
			}
			m := parseTelegramMessage(&update)
			t.ch <- m
		}
	}

}

func (t *telegramReceiver) SendMessage(message string, replyTo Message) error {
	msg := tgbotapi.NewMessage(replyTo.ChatID, message)
	if _, err := t.bot.Send(msg); err != nil {
		return err
	}
	return nil
}
