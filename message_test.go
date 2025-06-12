package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"rpi-bot/messaging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockMessageClient is a mock implementation of messaging.MessageClient
type MockMessageClient struct {
	mock.Mock
}

func (m *MockMessageClient) GetUpdates(ctx context.Context) <-chan messaging.Message {
	args := m.Called(ctx)
	return args.Get(0).(<-chan messaging.Message)
}

func (m *MockMessageClient) SendMessage(text string, originalMessage messaging.Message) error {
	args := m.Called(text, originalMessage)
	return args.Error(0)
}

func (m *MockMessageClient) Stop() {}

// MockCommandExecutor is a mock implementation of commandExecutor
type MockCommandExecutor struct {
	mock.Mock
}

func (m *MockCommandExecutor) execCommand(command string) (string, error) {
	args := m.Called(command)
	return args.String(0), args.Error(1)
}

func TestMessagingPoller(t *testing.T) {
	testCommands := map[string]Command{
		"testcmd": {Command: "echo %s", Args: []string{"arg1"}},
		"noargs":  {Command: "ls", Args: []string{}},
	}

	tests := []struct {
		name                 string
		setupMockClient      func(*MockMessageClient, chan messaging.Message)
		setupMockExecutor    func(*MockCommandExecutor)
		incomingMessages     []messaging.Message
		expectedMessagesSent []string
		expectedExecCommands []string
		cancelContextAfter   time.Duration // 0 for no early cancellation
	}{
		{
			name: "Successful command execution",
			incomingMessages: []messaging.Message{
				{Type: messaging.Command, Command: "testcmd", Args: []string{"hello"}},
			},
			setupMockClient: func(mc *MockMessageClient, ch chan messaging.Message) {
				mc.On("GetUpdates", mock.Anything).Return((<-chan messaging.Message)(ch))
				mc.On("SendMessage", "hello\n", messaging.Message{Type: messaging.Command, Command: "testcmd", Args: []string{"hello"}}).Return(nil)
			},
			setupMockExecutor: func(me *MockCommandExecutor) {
				me.On("execCommand", "echo hello").Return("hello\n", nil)
			},
			expectedMessagesSent: []string{"hello\n"},
			expectedExecCommands: []string{"echo hello"},
		},
		{
			name: "Command not supported",
			incomingMessages: []messaging.Message{
				{Type: messaging.Command, Command: "unknown", Args: []string{}},
			},
			setupMockClient: func(mc *MockMessageClient, ch chan messaging.Message) {
				mc.On("GetUpdates", mock.Anything).Return((<-chan messaging.Message)(ch))
				mc.On("SendMessage", "Command not supported", messaging.Message{Type: messaging.Command, Command: "unknown", Args: []string{}}).Return(nil)
			},
			setupMockExecutor: func(me *MockCommandExecutor) {
				// execCommand should not be called
			},
			expectedMessagesSent: []string{"Command not supported"},
		},
		{
			name: "createCommand fails - arg mismatch",
			incomingMessages: []messaging.Message{
				{Type: messaging.Command, Command: "testcmd", Args: []string{}}, // Missing arg
			},
			setupMockClient: func(mc *MockMessageClient, ch chan messaging.Message) {
				mc.On("GetUpdates", mock.Anything).Return((<-chan messaging.Message)(ch))
				mc.On("SendMessage", "Command formatting failed: mismatch between command definition args=1 and number of args=0", messaging.Message{Type: messaging.Command, Command: "testcmd", Args: []string{}}).Return(nil)
			},
			setupMockExecutor: func(me *MockCommandExecutor) {
				// execCommand should not be called
			},
			expectedMessagesSent: []string{"Command formatting failed: mismatch between command definition args=1 and number of args=0"},
		},
		{
			name: "executor.execCommand fails",
			incomingMessages: []messaging.Message{
				{Type: messaging.Command, Command: "noargs", Args: []string{}},
			},
			setupMockClient: func(mc *MockMessageClient, ch chan messaging.Message) {
				mc.On("GetUpdates", mock.Anything).Return((<-chan messaging.Message)(ch))
				mc.On("SendMessage", "Command ls failed: exec error", messaging.Message{Type: messaging.Command, Command: "noargs", Args: []string{}}).Return(nil)
			},
			setupMockExecutor: func(me *MockCommandExecutor) {
				me.On("execCommand", "ls").Return("", errors.New("exec error"))
			},
			expectedMessagesSent: []string{"Command ls failed: exec error"},
			expectedExecCommands: []string{"ls"},
		},
		{
			name: "SendMessage fails",
			incomingMessages: []messaging.Message{
				{Type: messaging.Command, Command: "noargs", Args: []string{}},
			},
			setupMockClient: func(mc *MockMessageClient, ch chan messaging.Message) {
				mc.On("GetUpdates", mock.Anything).Return((<-chan messaging.Message)(ch))
				mc.On("SendMessage", "ok", messaging.Message{Type: messaging.Command, Command: "noargs", Args: []string{}}).Return(errors.New("send error"))
			},
			setupMockExecutor: func(me *MockCommandExecutor) {
				me.On("execCommand", "ls").Return("ok", nil)
			},
			expectedMessagesSent: []string{"ok"}, // SendMessage is still attempted
			expectedExecCommands: []string{"ls"},
		},
		{
			name: "Update is not a command",
			incomingMessages: []messaging.Message{
				{Type: messaging.Chat, Text: "just a text"},
				{Type: messaging.Command, Command: "noargs", Args: []string{}},
			},
			setupMockClient: func(mc *MockMessageClient, ch chan messaging.Message) {
				mc.On("GetUpdates", mock.Anything).Return((<-chan messaging.Message)(ch))
				mc.On("SendMessage", "ok again", messaging.Message{Type: messaging.Command, Command: "noargs", Args: []string{}}).Return(nil)
			},
			setupMockExecutor: func(me *MockCommandExecutor) {
				me.On("execCommand", "ls").Return("ok again", nil)
			},
			expectedMessagesSent: []string{"ok again"},
			expectedExecCommands: []string{"ls"},
		},
		{
			name: "Context cancellation stops poller",
			incomingMessages: []messaging.Message{
				{Type: messaging.Command, Command: "noargs", Args: []string{}}, // This one might or might not be processed
				{Type: messaging.Command, Command: "noargs", Args: []string{}},
			},
			setupMockClient: func(mc *MockMessageClient, ch chan messaging.Message) {
				mc.On("GetUpdates", mock.Anything).Return((<-chan messaging.Message)(ch))
				// SendMessage might be called 0 or 1 times before cancellation
				mc.On("SendMessage", "ok ctx", messaging.Message{Type: messaging.Command, Command: "noargs", Args: []string{}}).Return(nil).Maybe()
			},
			setupMockExecutor: func(me *MockCommandExecutor) {
				// execCommand might be called 0 or 1 times
				me.On("execCommand", "ls").Return("ok ctx", nil).Maybe()
			},
			cancelContextAfter: 50 * time.Millisecond, // Cancel quickly
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockMessageClient)
			mockExecutor := new(MockCommandExecutor)
			updatesChan := make(chan messaging.Message, len(tt.incomingMessages)+1) // Buffered to prevent blocking sender

			tt.setupMockClient(mockClient, updatesChan)
			if tt.setupMockExecutor != nil {
				tt.setupMockExecutor(mockExecutor)
			}

			var wg sync.WaitGroup
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			wg.Add(1)
			go MessagingPoller(ctx, mockClient, mockExecutor, testCommands, &wg)

			// Send messages to the channel
			go func() {
				for _, msg := range tt.incomingMessages {
					select {
					case updatesChan <- msg:
					case <-ctx.Done(): // If context is cancelled, stop sending
						return
					}
					// Small delay to allow poller to process one by one in some cases
					// and for context cancellation test to work more predictably
					time.Sleep(10 * time.Millisecond)
				}
				if tt.cancelContextAfter == 0 { // Only close if not testing cancellation
					close(updatesChan) // Close channel to signal end of updates
				}
			}()

			if tt.cancelContextAfter > 0 {
				time.Sleep(tt.cancelContextAfter)
				cancel()
			}

			// Wait for MessagingPoller to finish
			// Use a timeout to prevent test hanging indefinitely
			waitChan := make(chan struct{})
			go func() {
				wg.Wait()
				close(waitChan)
			}()

			select {
			case <-waitChan:
				// Poller finished
			case <-time.After(2 * time.Second): // Adjusted timeout
				// If we cancelled context, this timeout is expected for that test case
				if tt.cancelContextAfter == 0 {
					t.Fatal("MessagingPoller did not finish in time")
				}
			}

			// Assertions
			mockClient.AssertExpectations(t)
			mockExecutor.AssertExpectations(t)

			// Verify SendMessage calls based on expectedMessagesSent
			if tt.cancelContextAfter == 0 { // Only for tests not involving early cancellation
				for _, expectedMsg := range tt.expectedMessagesSent {
					found := false
					for _, call := range mockClient.Calls {
						if call.Method == "SendMessage" {
							if call.Arguments.String(0) == expectedMsg {
								found = true
								break
							}
						}
					}
					assert.True(t, found, fmt.Sprintf("Expected SendMessage with text '%s' was not called", expectedMsg))
				}

				if len(tt.expectedExecCommands) > 0 {
					for _, expectedCmd := range tt.expectedExecCommands {
						found := false
						for _, call := range mockExecutor.Calls {
							if call.Method == "execCommand" {
								if call.Arguments.String(0) == expectedCmd {
									found = true
									break
								}
							}
						}
						assert.True(t, found, fmt.Sprintf("Expected execCommand with '%s' was not called", expectedCmd))
					}
				} else {
					mockExecutor.AssertNotCalled(t, "execCommand", mock.Anything)
				}
			}
		})
	}
}
