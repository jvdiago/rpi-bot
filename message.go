package main

import (
	"context"
	"fmt"
	"log"
	"rpi-bot/messaging"
	"sync"
)

func MessagingFactory(cfg *Config) (messaging.MessageClient, error) {
	var sr messaging.MessageClient

	if cfg.Provider == "telegram" {
		telegramApitoken, exists := GetSecret("TELEGRAM_APITOKEN", cfg.Telegram.ApiToken)
		if !exists {
			return sr, fmt.Errorf("ENV var `TELEGRAM_APITOKEN` not found")
		}
		return messaging.NewTelegramReceiver(telegramApitoken, cfg.Telegram.Debug)
	}
	if cfg.Provider == "signal" {
		return messaging.NewSignalReceiver(cfg.Signal.Socket, cfg.Signal.Sources)
	}
	if cfg.Provider == "" { // No messaging provider
		return sr, nil
	}
	return sr, fmt.Errorf("Provider %s not supportted", cfg.Provider)
}

func MessagingPoller(
	ctx context.Context,
	sr messaging.MessageClient,
	executor commandExecutor,
	commands map[string]Command,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	updates := sr.GetUpdates(ctx)

	for update := range updates {
		if update.Type != messaging.Command {
			continue
		}
		command, ok := commands[update.Command]

		var msg string
		if !ok {
			msg = "Command not supported"
		} else {
			fmtCommand, err := createCommand(command, update)
			if err != nil {
				msg = fmt.Sprintf("Command formatting failed: %v", err)

			} else {
				output, err := executor.execCommand(fmtCommand)
				if err != nil {
					msg = fmt.Sprintf("Command %s failed: %v", fmtCommand, err)
				} else {
					msg = output
				}
			}
		}
		err := sr.SendMessage(msg, update)

		if err != nil {
			log.Printf("Error sending a message: %v", err)
		}
	}

}
