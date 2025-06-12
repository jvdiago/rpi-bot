package main

import (
	"fmt"
	"log"
	"strings"

	"os/exec"
	"rpi-bot/messaging"
)

type commandExecutor interface {
	execCommand(command string) (string, error)
}

type executor struct{}

func (e *executor) execCommand(command string) (string, error) {
	parsedCommand := strings.Split(command, " ")
	if len(parsedCommand) == 0 {
		return "", fmt.Errorf("empty command")
	}

	cmd := exec.Command(parsedCommand[0], parsedCommand[1:]...)

	output, err := cmd.CombinedOutput()

	log.Printf("Executed command %s. result: %s. Err %s", command, output, err)
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func createCommand(c Command, m messaging.Message) (string, error) {
	if len(m.Args) != len(c.Args) {
		return "", fmt.Errorf(
			"mismatch between command definition args=%d and number of args=%d",
			len(c.Args), len(m.Args),
		)
	}

	placeholderCount := strings.Count(c.Command, "%s")

	if placeholderCount != len(c.Args) {
		return "", fmt.Errorf(
			"mismatch between placeholders (%%s)=%d and number of args=%d",
			placeholderCount, len(c.Args),
		)
	}

	if len(c.Args) == 0 {
		return c.Command, nil
	}

	// Convert []string → []interface{} so we can do `args...` in Sprintf.
	iface := make([]interface{}, len(m.Args))
	for i, s := range m.Args {
		iface[i] = s
	}

	return fmt.Sprintf(c.Command, iface...), nil
}
