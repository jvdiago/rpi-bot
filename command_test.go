package main

import (
	"rpi-bot/messaging"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateCommand(t *testing.T) {
	tests := []struct {
		name        string
		commandDef  Command
		message     messaging.Message
		expectedCmd string
		expectError bool
		errorMsg    string
	}{
		{
			name: "No arguments",
			commandDef: Command{
				Command: "ls -l",
				Args:    []string{},
			},
			message: messaging.Message{
				Args: []string{},
			},
			expectedCmd: "ls -l",
			expectError: false,
		},
		{
			name: "One argument",
			commandDef: Command{
				Command: "echo %s",
				Args:    []string{"text"},
			},
			message: messaging.Message{
				Args: []string{"hello"},
			},
			expectedCmd: "echo hello",
			expectError: false,
		},
		{
			name: "Multiple arguments",
			commandDef: Command{
				Command: "grep %s %s",
				Args:    []string{"pattern", "file"},
			},
			message: messaging.Message{
				Args: []string{"search_term", "my_file.txt"},
			},
			expectedCmd: "grep search_term my_file.txt",
			expectError: false,
		},
		{
			name: "Argument mismatch - too few provided",
			commandDef: Command{
				Command: "some_command %s %s",
				Args:    []string{"arg1", "arg2"},
			},
			message: messaging.Message{
				Args: []string{"only_one"},
			},
			expectedCmd: "",
			expectError: true,
			errorMsg:    "mismatch between command definition args=2 and number of args=1",
		},
		{
			name: "Argument mismatch - too many provided",
			commandDef: Command{
				Command: "another_command %s",
				Args:    []string{"arg1"},
			},
			message: messaging.Message{
				Args: []string{"val1", "val2"},
			},
			expectedCmd: "",
			expectError: true,
			errorMsg:    "mismatch between command definition args=1 and number of args=2",
		},
		{
			name: "Command definition has no args, but message provides some (should still work)",
			commandDef: Command{
				Command: "uptime",
				Args:    []string{},
			},
			message: messaging.Message{
				Args: []string{"ignored_arg"}, // This will cause a mismatch if not handled by arg count check
			},
			expectedCmd: "", // Expect error due to arg count mismatch
			expectError: true,
			errorMsg:    "mismatch between command definition args=0 and number of args=1",
		},
		{
			name: "Command definition has args, message provides none",
			commandDef: Command{
				Command: "ping %s",
				Args:    []string{"host"},
			},
			message: messaging.Message{
				Args: []string{},
			},
			expectedCmd: "",
			expectError: true,
			errorMsg:    "mismatch between command definition args=1 and number of args=0",
		},
		{
			name: "Command with no format specifiers but expects args (should treat command as literal)",
			commandDef: Command{
				Command: "fixed_command_with_args", // No %s
				Args:    []string{"placeholder1"},  // Definition expects one arg
			},
			message: messaging.Message{
				Args: []string{"actual_arg1"},
			},
			expectError: true,
			errorMsg:    "mismatch between placeholders (%s)=0 and number of args=1",
		},
	}
	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			cmdStr, err := createCommand(tt.commandDef, tt.message)

			if tt.expectError {
				require.Error(t, err, "expected an error")
				require.EqualError(t, err, tt.errorMsg)
			} else {
				require.NoError(t, err, "did not expect an error")
				require.Equal(t, tt.expectedCmd, cmdStr, "command string mismatch")
			}
		})
	}
}
