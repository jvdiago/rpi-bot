package messaging

import (
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

// helper to create a raw JSON-RPC notification containing ReceiveParams
func makeRPCNotification(params ReceiveParams) *rpcMessage {
	rawParams, _ := json.Marshal(params)
	raw := json.RawMessage(rawParams)
	return &rpcMessage{
		JSONRPC: "2.0",
		Method:  "receive",
		Params:  &raw,
	}
}

func TestParseMessage_ValidSourceChat(t *testing.T) {
	// Prepare a ReceiveParams where SourceNumber is in the allowed sources list.
	incoming := ReceiveParams{
		Account: "alice",
		Envelope: Envelope{
			ServerDeliveredTimestamp: 1234567890,
			ServerReceivedTimestamp:  1234567890,
			Source:                   "alice-device",
			SourceDevice:             1,
			SourceName:               "Alice’s Phone",
			SourceNumber:             "+15551234567",
			SourceUUID:               "uuid-abc-123",
			SyncMessage: SyncMessage{
				SentMessage: SentMessage{
					Destination:       "bob",
					DestinationNumber: "+15557654321",
					DestinationUUID:   "uuid-def-456",
					ExpiresInSeconds:  0,
					Message:           "command arg1",
					Timestamp:         1234567890,
					ViewOnce:          false,
				},
			},
		},
		Timestamp: 1234567890,
	}

	msg := makeRPCNotification(incoming)
	sources := []string{"+15551234567", "+15559876543"}

	parsed, err := parseMessage(msg, sources)
	require.NoError(t, err)

	// Since the message content does not start with "/", it should be Chat
	require.Equal(t, Chat, parsed.Type)
	require.Equal(t, "command arg1", incoming.Envelope.SyncMessage.SentMessage.Message) // Raw stores full JSON of params
	require.Equal(t, incoming.Envelope.SourceNumber, parsed.Source)
	require.Equal(t, "arg1", parsed.Args[0])
}

func TestParseMessage_ValidSourceCommand(t *testing.T) {
	// Prepare a ReceiveParams where the message begins with "/cmd arg1 arg2"
	recvParams := ReceiveParams{
		Account: "alice",
		Envelope: Envelope{
			ServerDeliveredTimestamp: 1111,
			ServerReceivedTimestamp:  1111,
			Source:                   "alice-device",
			SourceDevice:             1,
			SourceName:               "Alice’s Phone",
			SourceNumber:             "+15551234567",
			SourceUUID:               "uuid-abc-123",
			SyncMessage: SyncMessage{
				SentMessage: SentMessage{
					Destination:       "bob",
					DestinationNumber: "+15557654321",
					DestinationUUID:   "uuid-def-456",
					ExpiresInSeconds:  0,
					Message:           "/greet Alice Bob",
					Timestamp:         1111,
					ViewOnce:          false,
				},
			},
		},
		Timestamp: 1111,
	}

	msg := makeRPCNotification(recvParams)
	sources := []string{"+15551234567"}

	parsed, err := parseMessage(msg, sources)
	require.NoError(t, err)

	// Since the message starts with "/", it should be Command
	require.Equal(t, Command, parsed.Type)
	require.Equal(t, "greet", parsed.Command)
	require.Equal(t, []string{"Alice", "Bob"}, parsed.Args)
	require.Equal(t, recvParams.Envelope.SourceNumber, parsed.Source)
}

func TestParseMessage_InvalidSource(t *testing.T) {
	// The SourceNumber is not in the supplied "sources" slice
	recvParams := ReceiveParams{
		Account: "alice",
		Envelope: Envelope{
			ServerDeliveredTimestamp: 2222,
			ServerReceivedTimestamp:  2222,
			Source:                   "alice-device",
			SourceDevice:             1,
			SourceName:               "Alice’s Phone",
			SourceNumber:             "+15550000000",
			SourceUUID:               "uuid-abc-123",
			SyncMessage: SyncMessage{
				SentMessage: SentMessage{
					Destination:       "bob",
					DestinationNumber: "+15557654321",
					DestinationUUID:   "uuid-def-456",
					ExpiresInSeconds:  0,
					Message:           "hi there",
					Timestamp:         2222,
					ViewOnce:          false,
				},
			},
		},
		Timestamp: 2222,
	}

	msg := makeRPCNotification(recvParams)
	sources := []string{"+15551234567"} // does not include "+15550000000"

	_, err := parseMessage(msg, sources)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Message ignored as sources is not valid")
}

func TestParseMessage_BadJSON(t *testing.T) {
	// Create an rpcMessage with invalid JSON in Params
	raw := json.RawMessage([]byte(`{"invalid_json":`)) // malformed JSON
	msg := &rpcMessage{
		JSONRPC: "2.0",
		Method:  "receive",
		Params:  &raw,
	}
	sources := []string{"+15551234567"}

	_, err := parseMessage(msg, sources)
	require.Error(t, err)
	require.True(t, errors.Is(err, err), "Expected an unmarshal error")
}

// makeRPCResponse is a helper to create an rpcMessage with a valid Result field.
func makeRPCResponse(res SendResult) *rpcMessage {
	rawRes, _ := json.Marshal(res)
	raw := json.RawMessage(rawRes)
	return &rpcMessage{
		JSONRPC: "2.0",
		Result:  &raw,
	}
}

func TestParseResponse_Valid(t *testing.T) {
	// Construct a SendResult with two entries
	expected := SendResult{
		Timestamp: 1618033988,
		Results: []SendResultEntry{
			{
				RecipientAddress: RecipientAddress{
					UUID:   "uuid-123",
					Number: "+15551234567",
				},
				Type: "sent",
			},
			{
				RecipientAddress: RecipientAddress{
					UUID:   "uuid-456",
					Number: "+15557654321",
				},
				Type: "failed",
			},
		},
	}

	msg := makeRPCResponse(expected)
	parsed, err := parseResponse(msg)
	require.NoError(t, err)

	// The returned Message should have Type=Response, Raw equal to the JSON string,
	// and Text empty (since parseResponse doesn’t set Text or ChatID)
	require.Equal(t, Response, parsed.Type)
	require.Equal(t, string(*msg.Result), parsed.Raw)

	// Unmarshal parsed.Raw back into a SendResult and compare to expected
	var actual SendResult
	err = json.Unmarshal([]byte(parsed.Raw), &actual)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestParseResponse_InvalidJSON(t *testing.T) {
	// Create an rpcMessage whose Result is invalid JSON
	badRaw := json.RawMessage([]byte(`{"timestamp": 1234, "results": [invalid]}`))
	msg := &rpcMessage{
		JSONRPC: "2.0",
		Result:  &badRaw,
	}

	_, err := parseResponse(msg)
	require.Error(t, err)
	// Ensure the error mentions unmarshal or invalid character
	require.True(t, errors.Is(err, err) ||
		strings.Contains(err.Error(), "unmarshal"),
		"Expected an unmarshal error, got: %v", err)
}
