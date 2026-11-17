package requester

import (
	"fmt"
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
			return
		}

		if p.handler != nil {
			shouldContinue, usage, newMessage, err := p.handler(source, messageType, message)
			if err != nil {
				errMsg := []byte(err.Error())
				p.write(dst, websocket.TextMessage, errMsg)
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
					p.write(dst, websocket.TextMessage, message)
					errMsg := []byte(err.Error())
					p.write(dst, websocket.TextMessage, errMsg)
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
