package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"slices"
	"strings"
	"sync"
)

type rpcMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	Method  string           `json:"method,omitempty"` // for notifications
	Params  *json.RawMessage `json:"params,omitempty"` // notification payload
	Result  *json.RawMessage `json:"result,omitempty"` // response payload
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
	ID *int `json:"id,omitempty"` // pointer so we can tell “absent” vs “0”
}

// rpcRequest is a JSON-RPC 2.0 request envelope
type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"` // must be "2.0"
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int         `json:"id"`
}

// sendParams matches signal-cli’s expected named params for "send"
type sendParams struct {
	Recipient []string `json:"recipient"`
	Message   string   `json:"message"`
}

// ---  “send” RPC: result payload ---
type SendResult struct {
	Timestamp int64             `json:"timestamp"`
	Results   []SendResultEntry `json:"results"`
}

type SendResultEntry struct {
	RecipientAddress RecipientAddress `json:"recipientAddress"`
	Type             string           `json:"type"`
}

type RecipientAddress struct {
	UUID   string `json:"uuid"`
	Number string `json:"number"`
}

// ---  “receive” notification: params payload ---
type ReceiveParams struct {
	Account   string   `json:"account"`
	Envelope  Envelope `json:"envelope"`
	Timestamp int64    `json:"timestamp"`
}

type Envelope struct {
	ServerDeliveredTimestamp int64       `json:"serverDeliveredTimestamp"`
	ServerReceivedTimestamp  int64       `json:"serverReceivedTimestamp"`
	Source                   string      `json:"source"`
	SourceDevice             int         `json:"sourceDevice"`
	SourceName               string      `json:"sourceName"`
	SourceNumber             string      `json:"sourceNumber"`
	SourceUUID               string      `json:"sourceUuid"`
	SyncMessage              SyncMessage `json:"syncMessage"`
}

type SyncMessage struct {
	SentMessage SentMessage `json:"sentMessage"`
}

type SentMessage struct {
	Destination       string `json:"destination"`
	DestinationNumber string `json:"destinationNumber"`
	DestinationUUID   string `json:"destinationUuid"`
	ExpiresInSeconds  int    `json:"expiresInSeconds"`
	Message           string `json:"message"`
	Timestamp         int64  `json:"timestamp"`
	ViewOnce          bool   `json:"viewOnce"`
}
type signalReceiver struct {
	ch      chan Message
	sources []string
	socket  string
	enc     *json.Encoder
	dec     *json.Decoder
	conn    net.Conn
	nextID  int
	mu      sync.Mutex // protects enc.Encode
}

func NewSignalReceiver(socketPath string, sources []string) (*signalReceiver, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}
	r := &signalReceiver{
		ch:      make(chan Message, 10),
		conn:    conn,
		enc:     json.NewEncoder(conn),
		dec:     json.NewDecoder(conn),
		socket:  socketPath,
		sources: sources,
		nextID:  1,
	}
	return r, nil
}

func (s *signalReceiver) GetUpdates(ctx context.Context) <-chan Message {
	go s.messageReceiver(ctx)
	return s.ch
}

func parseMessage(msg *rpcMessage, sources []string) (Message, error) {
	var recvParams ReceiveParams
	var message Message
	if err := json.Unmarshal(*msg.Params, &recvParams); err != nil {
		err = fmt.Errorf("messageReceiver: unmarshal notification error to interface map: %v", err)
		return message, err
	}
	if !slices.Contains(sources, recvParams.Envelope.SourceNumber) {
		err := fmt.Errorf("messageReceiver: Message ignored as sources is not valid: %s", recvParams.Envelope.SourceNumber)
		return message, err
	}

	commands := strings.Split(recvParams.Envelope.SyncMessage.SentMessage.Message, " ")
	command := commands[0]
	messageType := Chat
	if strings.HasPrefix(command, "/") {
		messageType = Command
		command = strings.TrimPrefix(command, "/")
	}
	message = Message{
		Type:    messageType,
		Raw:     string(*msg.Params),
		Command: command,
		Source:  recvParams.Envelope.SourceNumber,
		Args:    commands[1:],
	}
	return message, nil
}

func parseResponse(msg *rpcMessage) (Message, error) {
	var sendRes SendResult
	var message Message
	if err := json.Unmarshal(*msg.Result, &sendRes); err != nil {
		err = fmt.Errorf("messageReceiver: unmarshal response error to interface map: %v", err)
		return message, err
	}
	message = Message{Type: Response, Raw: string(*msg.Result)}
	return message, nil

}
func (s *signalReceiver) messageReceiver(ctx context.Context) {
	go func() {
		<-ctx.Done()
		log.Println("messageReceiver: context done, closing connection")
		s.conn.Close()
	}()
	defer close(s.ch)

	for {
		select {
		case <-ctx.Done():
			log.Println("messageReceiver: context done, exiting loop")
			return
		default:
		}
		var msg rpcMessage
		if err := s.dec.Decode(&msg); err != nil {
			if errors.Is(err, io.EOF) {
				log.Println("messageReceiver: socket closed (EOF), exiting")
				return
			}
			if errors.Is(err, net.ErrClosed) {
				log.Printf("messageReceiver: connection closed (net.ErrClosed), exiting")
				return
			}
		}

		// RPC-level error
		if msg.Error != nil {
			log.Printf("messageReceiver: RPC error (id=%v): %s", msg.ID, msg.Error.Message)
			continue
		}

		// Message type: New Message
		if msg.Params != nil && msg.Method == "receive" {
			m, err := parseMessage(&msg, s.sources)
			if err != nil {
				log.Println(err)
				continue
			}
			s.ch <- m
			continue
		}

		// Message type: Response
		if msg.Result != nil {
			m, err := parseResponse(&msg)
			if err != nil {
				log.Println(err)
				continue
			}
			s.ch <- m
			continue
		}

		// Everything else
		log.Printf("messageReceiver: unrecognized message: %+v", msg)
	}
}

func (s *signalReceiver) getNextID() int {
	s.mu.Lock()
	id := s.nextID
	s.nextID++
	s.mu.Unlock()

	return id
}

func (s *signalReceiver) SendMessage(message string, replyTo Message) error {
	id := s.getNextID()

	req := rpcRequest{
		JSONRPC: "2.0",
		Method:  "send",
		Params: sendParams{
			Recipient: []string{replyTo.Source},
			Message:   message,
		},
		ID: id,
	}
	if err := s.enc.Encode(req); err != nil {
		return fmt.Errorf("encode error: %v", err)
	}
	return nil
}
