package requester

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/types"
)

type WSProxy struct {
	userConn       *websocket.Conn
	supplierConn   *websocket.Conn
	timeout        time.Duration
	handler        MessageHandler
	usageHandler   UsageHandler
	done           chan struct{}
	userClosed     chan struct{}
	supplierClosed chan struct{}
	// A websocket connection allows one writer at a time; these serialize the proxy's own
	// writes with WriteToUser calls made from a MessageHandler.
	userWriteMu     sync.Mutex
	supplierWriteMu sync.Mutex
	// closeFrames reports handler and usage errors as close frames instead of text messages
	// (the Gemini Live protocol has no error event).
	closeFrames bool
}

type MessageSource int

const (
	UserMessage MessageSource = iota
	SupplierMessage
)

type MessageHandler func(source MessageSource, messageType int, message []byte) (bool, *types.UsageEvent, []byte, error)
type UsageHandler func(usage *types.UsageEvent) error

func NewWSProxy(userConn, supplierConn *websocket.Conn, timeout time.Duration, handler MessageHandler, usageHandler UsageHandler) *WSProxy {
	return &WSProxy{
		userConn:       userConn,
		supplierConn:   supplierConn,
		timeout:        timeout,
		handler:        handler,
		usageHandler:   usageHandler,
		done:           make(chan struct{}),
		userClosed:     make(chan struct{}),
		supplierClosed: make(chan struct{}),
	}
}

// UseCloseFrames makes the proxy report handler and usage errors to the other side as a close
// frame (policy violation) carrying the error text, instead of a text message.
func (p *WSProxy) UseCloseFrames() {
	p.closeFrames = true
}

func (p *WSProxy) Start() {
	go p.transfer(p.userConn, p.supplierConn, UserMessage, p.userClosed)
	go p.transfer(p.supplierConn, p.userConn, SupplierMessage, p.supplierClosed)
}

func (p *WSProxy) Wait() {
	<-p.done
}

func (p *WSProxy) Close() {
	p.userConn.Close()
	p.supplierConn.Close()
}

func (p *WSProxy) UserClosed() <-chan struct{} {
	return p.userClosed
}

func (p *WSProxy) SupplierClosed() <-chan struct{} {
	return p.supplierClosed
}

// WriteToUser sends a message to the user connection. It is safe to call from a MessageHandler
// (for example to answer a client event that is not forwarded) while the proxy runs.
func (p *WSProxy) WriteToUser(messageType int, data []byte) error {
	return p.write(p.userConn, messageType, data)
}

func (p *WSProxy) write(conn *websocket.Conn, messageType int, data []byte) error {
	mu := &p.supplierWriteMu
	if conn == p.userConn {
		mu = &p.userWriteMu
	}
	mu.Lock()
	defer mu.Unlock()
	return conn.WriteMessage(messageType, data)
}

func (p *WSProxy) reportError(dst *websocket.Conn, err error) {
	if p.closeFrames {
		p.writeClose(dst, websocket.ClosePolicyViolation, err.Error())
		return
	}
	p.write(dst, websocket.TextMessage, []byte(err.Error()))
}

// sendableCloseCode reports whether a close code received from a peer may be sent on: 1005, 1006
// and 1015 describe the local end of a connection and must never appear in a close frame.
func sendableCloseCode(code int) bool {
	switch code {
	case websocket.CloseNoStatusReceived, websocket.CloseAbnormalClosure, websocket.CloseTLSHandshake:
		return false
	}
	return true
}

// writeClose sends a close frame; the reason is cut to the 123 bytes a close frame holds.
func (p *WSProxy) writeClose(dst *websocket.Conn, code int, reason string) {
	if len(reason) > 123 {
		reason = strings.ToValidUTF8(reason[:123], "")
	}
	_ = dst.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(time.Second))
}

func (p *WSProxy) transfer(src, dst *websocket.Conn, source MessageSource, closed chan<- struct{}) {
	defer func() {
		close(closed)
		p.done <- struct{}{}
	}()

	for {
		src.SetReadDeadline(time.Now().Add(p.timeout))

		messageType, message, err := src.ReadMessage()
		if err != nil {
			logger.SysError(fmt.Sprintf("source: %d, ReadMessage error: %s", source, err.Error()))
			// Pass a close frame on, so the other side learns why the session ended (a session
			// limit, an invalid message) instead of seeing an abnormal closure.
			var closeErr *websocket.CloseError
			if errors.As(err, &closeErr) && sendableCloseCode(closeErr.Code) {
				p.writeClose(dst, closeErr.Code, closeErr.Text)
			}
			return
		}

		if p.handler != nil {
			shouldContinue, usage, newMessage, err := p.handler(source, messageType, message)
			if err != nil {
				p.reportError(dst, err)
				logger.SysError(fmt.Sprintf("source: %d, handler error: %s", source, err.Error()))
				return
			}

			if !shouldContinue {
				return
			}

			if newMessage != nil {
				message = newMessage
			}

			if usage != nil && p.usageHandler != nil {
				err := p.usageHandler(usage)
				if err != nil {
					p.write(dst, messageType, message)
					p.reportError(dst, err)
					logger.SysError(fmt.Sprintf("source: %d, usageHandler error: %s", source, err.Error()))
					return
				}
			}
		}

		err = p.write(dst, messageType, message)
		if err != nil {
			logger.SysError(fmt.Sprintf("source: %d, WriteMessage error: %s", source, err.Error()))
			return
		}
	}
}
