package lsp

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/yukin/kore/pkg/logger"
)

func TestJSONRPC2(t *testing.T) {
	log := logger.New(os.Stdout, os.Stderr, logger.DEBUG, "")

	// Create in-memory pipes
	serverIn := new(bytes.Buffer)
	clientIn := new(bytes.Buffer)

	server := NewJSONRPC2(clientIn, serverIn, log)
	client := NewJSONRPC2(serverIn, clientIn, log)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start server
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Close()

	// Register server handler
	server.Handle("test", func(ctx context.Context, params interface{}) (interface{}, error) {
		return map[string]interface{}{
			"message": "hello",
			"params":  params,
		}, nil
	})

	// Test request
	var result map[string]interface{}
	err := client.Request(ctx, "test", map[string]string{"foo": "bar"}, &result)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if result["message"] != "hello" {
		t.Errorf("Expected 'hello', got '%v'", result["message"])
	}

	// Test notification
	err = client.Notify("testNotification", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("Notification failed: %v", err)
	}
}

func TestReadContentLength(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedLength int
		expectError    bool
	}{
		{
			name:           "valid header",
			input:          "Content-Length: 42\r\n\r\n",
			expectedLength: 42,
			expectError:    false,
		},
		{
			name:           "valid header with spaces",
			input:          "Content-Length:  123 \r\n\r\n",
			expectedLength: 123,
			expectError:    false,
		},
		{
			name:           "no content-length",
			input:          "Content-Type: application/json\r\n\r\n",
			expectedLength: 0,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewBufferString(tt.input)
			length, err := readContentLength(reader)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if length != tt.expectedLength {
				t.Errorf("Expected length %d, got %d", tt.expectedLength, length)
			}
		})
	}
}

func TestMessageParsing(t *testing.T) {
	log := logger.New(os.Stdout, os.Stderr, logger.ERROR, "")

	tests := []struct {
		name      string
		input     string
		checkFunc func(*testing.T, *Message)
		expectErr bool
	}{
		{
			name: "valid request",
			input: `{"jsonrpc":"2.0","id":1,"method":"test","params":{}}`,
			checkFunc: func(t *testing.T, msg *Message) {
				if msg.Request == nil {
					t.Error("Expected request message")
					return
				}
				if msg.Request.Method != "test" {
					t.Errorf("Expected method 'test', got '%s'", msg.Request.Method)
				}
			},
			expectErr: false,
		},
		{
			name: "valid response",
			input: `{"jsonrpc":"2.0","id":1,"result":{"status":"ok"}}`,
			checkFunc: func(t *testing.T, msg *Message) {
				if msg.Response == nil {
					t.Error("Expected response message")
					return
				}
				if msg.Response.Result == nil {
					t.Error("Expected result")
				}
			},
			expectErr: false,
		},
		{
			name: "valid notification",
			input: `{"jsonrpc":"2.0","method":"notification","params":{}}`,
			checkFunc: func(t *testing.T, msg *Message) {
				if msg.Notification == nil {
					t.Error("Expected notification message")
					return
				}
				if msg.Notification.Method != "notification" {
					t.Errorf("Expected method 'notification', got '%s'", msg.Notification.Method)
				}
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewJSONRPC2(nil, nil, log)

			// Simulate reading message
			var raw json.RawMessage = json.RawMessage(tt.input)
			var base struct {
				JSONRPC string `json:"jsonrpc"`
				ID      *ID    `json:"id"`
				Method  string `json:"method"`
			}

			if err := json.Unmarshal(raw, &base); err != nil {
				if !tt.expectErr {
					t.Errorf("Failed to parse base message: %v", err)
				}
				return
			}

			msg := &Message{}

			if base.ID != nil {
				if base.Method != "" {
					var req RequestMessage
					if err := json.Unmarshal(raw, &req); err != nil {
						t.Errorf("Failed to parse request: %v", err)
						return
					}
					msg.Request = &req
				} else {
					var resp ResponseMessage
					if err := json.Unmarshal(raw, &resp); err != nil {
						t.Errorf("Failed to parse response: %v", err)
						return
					}
					msg.Response = &resp
				}
			} else {
				var notif NotificationMessage
				if err := json.Unmarshal(raw, &notif); err != nil {
					t.Errorf("Failed to parse notification: %v", err)
					return
				}
				msg.Notification = &notif
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, msg)
			}
		})
	}
}

func TestWriteMessage(t *testing.T) {
	log := logger.New(os.Stdout, os.Stderr, logger.ERROR, "")
	buf := new(bytes.Buffer)
	client := NewJSONRPC2(nil, buf, log)

	data := []byte(`{"test":"data"}`)
	err := client.writeMessage(data)
	if err != nil {
		t.Fatalf("writeMessage failed: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("Expected output, got empty string")
	}

	// Check that it contains Content-Length header
	if !bytes.Contains(buf.Bytes(), []byte("Content-Length:")) {
		t.Error("Expected Content-Length header in output")
	}
}
