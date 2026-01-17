package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/yukin/kore/pkg/logger"
)

var (
	nextRequestID atomic.Int64
)

// JSONRPC2 implements JSON-RPC 2.0 client over stdio
type JSONRPC2 struct {
	log        *logger.Logger
	rw         *readWriter
	handlers   map[string]func(ctx context.Context, params interface{}) (interface{}, error)
	handlersMu sync.RWMutex

	pendingRequests map[ID]chan *ResponseMessage
	pendingMu       sync.RWMutex

	closed   atomic.Bool
	closeCh  chan struct{}
}

type readWriter struct {
	in  *bufio.Reader
	out io.Writer
}

// NewJSONRPC2 creates a new JSON-RPC 2.0 client
func NewJSONRPC2(in io.Reader, out io.Writer, log *logger.Logger) *JSONRPC2 {
	return &JSONRPC2{
		log:             log,
		rw:              &readWriter{in: bufio.NewReader(in), out: out},
		handlers:        make(map[string]func(ctx context.Context, params interface{}) (interface{}, error)),
		pendingRequests: make(map[ID]chan *ResponseMessage),
		closeCh:         make(chan struct{}),
	}
}

// Start starts processing messages
func (c *JSONRPC2) Start(ctx context.Context) error {
	go c.readLoop(ctx)
	return nil
}

// Close closes the connection
func (c *JSONRPC2) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil // already closed
	}
	close(c.closeCh)
	return nil
}

// readLoop reads messages from stdin
func (c *JSONRPC2) readLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			c.log.Debug("JSON-RPC read loop stopped: context done")
			return
		case <-c.closeCh:
			c.log.Debug("JSON-RPC read loop stopped: closed")
			return
		default:
		}

		msg, err := c.readMessage()
		if err != nil {
			if err == io.EOF {
				c.log.Debug("JSON-RPC connection closed (EOF)")
				return
			}
			c.log.Error("Failed to read JSON-RPC message: %v", err)
			return
		}

		// Process message based on type
		if msg.Request != nil {
			c.log.Debug("Received request: %s (ID: %d)", msg.Request.Method, msg.Request.ID)
			go c.handleRequest(ctx, msg.Request)
		} else if msg.Response != nil {
			c.log.Debug("Received response for ID: %d", msg.Response.ID)
			c.handleResponse(msg.Response)
		} else if msg.Notification != nil {
			c.log.Debug("Received notification: %s", msg.Notification.Method)
			go c.handleNotification(ctx, msg.Notification)
		}
	}
}

// Message represents a generic JSON-RPC message
type Message struct {
	Request      *RequestMessage
	Response     *ResponseMessage
	Notification *NotificationMessage
}

// readMessage reads a single JSON-RPC message
func (c *JSONRPC2) readMessage() (*Message, error) {
	// Read Content-Length header
	contentLength, err := readContentLength(c.rw.in)
	if err != nil {
		return nil, fmt.Errorf("failed to read Content-Length: %w", err)
	}

	// Read the JSON-RPC message
	line, err := c.rw.in.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read message: %w", err)
	}

	var raw json.RawMessage
	if err := json.Unmarshal(line, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Determine message type
	var base struct {
		JSONRPC string `json:"jsonrpc"`
		ID      *ID    `json:"id"`
		Method  string `json:"method"`
	}

	if err := json.Unmarshal(raw, &base); err != nil {
		return nil, fmt.Errorf("failed to parse base message: %w", err)
	}

	msg := &Message{}

	// Check if it's a request or response (has ID) or notification (no ID)
	if base.ID != nil {
		// Check if it has a method (request) or error/result (response)
		if base.Method != "" {
			// Request
			var req RequestMessage
			if err := json.Unmarshal(raw, &req); err != nil {
				return nil, fmt.Errorf("failed to parse request: %w", err)
			}
			msg.Request = &req
		} else {
			// Response
			var resp ResponseMessage
			if err := json.Unmarshal(raw, &resp); err != nil {
				return nil, fmt.Errorf("failed to parse response: %w", err)
			}
			msg.Response = &resp
		}
	} else {
		// Notification
		var notif NotificationMessage
		if err := json.Unmarshal(raw, &notif); err != nil {
			return nil, fmt.Errorf("failed to parse notification: %w", err)
		}
		msg.Notification = &notif
	}

	c.log.Debug("Read message with Content-Length: %d bytes", contentLength)
	return msg, nil
}

// readContentLength reads the Content-Length header
func readContentLength(reader *bufio.Reader) (int, error) {
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return 0, err
		}

		// Parse header
		if len(line) > 16 && line[:16] == "Content-Length: " {
			var length int
			_, err := fmt.Sscanf(line[16:], "%d", &length)
			if err != nil {
				return 0, fmt.Errorf("failed to parse Content-Length: %w", err)
			}
			return length, nil
		}

		// Empty line indicates end of headers
		if line == "\r\n" || line == "\n" {
			return 0, fmt.Errorf("no Content-Length header found")
		}
	}
}

// writeMessage writes a JSON-RPC message
func (c *JSONRPC2) writeMessage(data []byte) error {
	// Calculate Content-Length
	contentLength := len(data)

	// Write headers and body
	headers := fmt.Sprintf("Content-Length: %d\r\n\r\n", contentLength)
	fullMessage := headers + string(data)

	if _, err := c.rw.out.Write([]byte(fullMessage)); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// Request sends a JSON-RPC request and waits for response
func (c *JSONRPC2) Request(ctx context.Context, method string, params interface{}, result interface{}) error {
	if c.closed.Load() {
		return fmt.Errorf("connection closed")
	}

	// Generate unique request ID
	id := ID(nextRequestID.Add(1))

	// Create request
	req := &RequestMessage{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	// Marshal request
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create response channel
	respCh := make(chan *ResponseMessage, 1)
	c.pendingMu.Lock()
	c.pendingRequests[id] = respCh
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pendingRequests, id)
		c.pendingMu.Unlock()
		close(respCh)
	}()

	// Write request
	c.log.Debug("Sending request: %s (ID: %d)", method, id)
	if err := c.writeMessage(data); err != nil {
		return err
	}

	// Wait for response
	select {
	case resp := <-respCh:
		if resp.Error != nil {
			return fmt.Errorf("request failed (code %d): %s", resp.Error.Code, resp.Error.Message)
		}
		if result != nil && resp.Result != nil {
			// Marshal and unmarshal to convert to result type
			resultData, err := json.Marshal(resp.Result)
			if err != nil {
				return fmt.Errorf("failed to marshal result: %w", err)
			}
			if err := json.Unmarshal(resultData, result); err != nil {
				return fmt.Errorf("failed to unmarshal result: %w", err)
			}
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-c.closeCh:
		return fmt.Errorf("connection closed")
	}
}

// Notify sends a JSON-RPC notification
func (c *JSONRPC2) Notify(method string, params interface{}) error {
	if c.closed.Load() {
		return fmt.Errorf("connection closed")
	}

	// Create notification
	notif := &NotificationMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	// Marshal notification
	data, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// Write notification
	c.log.Debug("Sending notification: %s", method)
	return c.writeMessage(data)
}

// Handle registers a handler for a method
func (c *JSONRPC2) Handle(method string, handler func(ctx context.Context, params interface{}) (interface{}, error)) {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	c.handlers[method] = handler
}

// handleRequest handles an incoming request
func (c *JSONRPC2) handleRequest(ctx context.Context, req *RequestMessage) {
	c.handlersMu.RLock()
	handler, ok := c.handlers[req.Method]
	c.handlersMu.RUnlock()

	if !ok {
		c.sendError(req.ID, MethodNotFound, fmt.Sprintf("method not found: %s", req.Method))
		return
	}

	// Call handler
	result, err := handler(ctx, req.Params)
	if err != nil {
		c.sendError(req.ID, InternalError, err.Error())
		return
	}

	// Send response
	c.sendResponse(req.ID, result)
}

// handleNotification handles an incoming notification
func (c *JSONRPC2) handleNotification(ctx context.Context, notif *NotificationMessage) {
	c.handlersMu.RLock()
	handler, ok := c.handlers[notif.Method]
	c.handlersMu.RUnlock()

	if !ok {
		c.log.Warn("No handler for notification: %s", notif.Method)
		return
	}

	// Call handler (ignore result)
	_, _ = handler(ctx, notif.Params)
}

// handleResponse handles an incoming response
func (c *JSONRPC2) handleResponse(resp *ResponseMessage) {
	c.pendingMu.RLock()
	ch, ok := c.pendingRequests[resp.ID]
	c.pendingMu.RUnlock()

	if !ok {
		c.log.Warn("No pending request for response ID: %d", resp.ID)
		return
	}

	select {
	case ch <- resp:
	default:
		c.log.Warn("Response channel full for ID: %d", resp.ID)
	}
}

// sendResponse sends a response message
func (c *JSONRPC2) sendResponse(id ID, result interface{}) error {
	resp := &ResponseMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	return c.writeMessage(data)
}

// sendError sends an error response
func (c *JSONRPC2) sendError(id ID, code int, message string) error {
	resp := &ResponseMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: &ResponseError{
			Code:    code,
			Message: message,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	return c.writeMessage(data)
}
