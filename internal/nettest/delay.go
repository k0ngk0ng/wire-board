// Package nettest supplies opt-in latency for local network acceptance only.
// It is never imported by the application server or game implementations.
package nettest

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Delay adds RTT/2 in each direction to WebSocket bytes. Producers continue
// enqueueing while earlier bytes are in transit, preserving pipeline throughput.
// Ordinary HTTP receives one RTT before the response; assets share the same path.
func Delay(next http.Handler, rtt time.Duration) http.Handler {
	if rtt <= 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			next.ServeHTTP(&delayedWriter{ResponseWriter: w, oneWay: rtt / 2}, r)
			return
		}
		timer := time.NewTimer(rtt)
		defer timer.Stop()
		select {
		case <-timer.C:
			next.ServeHTTP(w, r)
		case <-r.Context().Done():
			return
		}
	})
}

type delayedWriter struct {
	http.ResponseWriter
	oneWay time.Duration
}

func (w *delayedWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, rw, err := w.ResponseWriter.(http.Hijacker).Hijack()
	if err != nil {
		return nil, nil, err
	}
	if err = rw.Flush(); err != nil {
		conn.Close()
		return nil, nil, err
	}
	c := newDelayedConn(conn, rw.Reader, w.oneWay)
	return c, bufio.NewReadWriter(bufio.NewReader(c), bufio.NewWriter(c)), nil
}

type delayedChunk struct {
	data []byte
	err  error
	at   time.Time
}
type delayedConn struct {
	net.Conn
	delay              time.Duration
	incoming, outgoing chan delayedChunk
	done               chan struct{}
	once               sync.Once
	readMu             sync.Mutex
	remaining          []byte
}

func newDelayedConn(conn net.Conn, source io.Reader, delay time.Duration) *delayedConn {
	c := &delayedConn{Conn: conn, delay: delay, incoming: make(chan delayedChunk, 128), outgoing: make(chan delayedChunk, 128), done: make(chan struct{})}
	go func() {
		for {
			buffer := make([]byte, 32*1024)
			n, err := source.Read(buffer)
			if n > 0 {
				select {
				case c.incoming <- delayedChunk{data: buffer[:n], at: time.Now().Add(delay)}:
				case <-c.done:
					return
				}
			}
			if err != nil {
				select {
				case c.incoming <- delayedChunk{err: err, at: time.Now().Add(delay)}:
				case <-c.done:
				}
				return
			}
		}
	}()
	go func() {
		for {
			select {
			case chunk := <-c.outgoing:
				if !c.wait(chunk.at) {
					return
				}
				for len(chunk.data) > 0 {
					n, err := conn.Write(chunk.data)
					if err != nil {
						c.Close()
						return
					}
					chunk.data = chunk.data[n:]
				}
			case <-c.done:
				return
			}
		}
	}()
	return c
}
func (c *delayedConn) wait(at time.Time) bool {
	timer := time.NewTimer(time.Until(at))
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-c.done:
		return false
	}
}
func (c *delayedConn) Read(p []byte) (int, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	if len(c.remaining) == 0 {
		select {
		case chunk := <-c.incoming:
			if !c.wait(chunk.at) {
				return 0, net.ErrClosed
			}
			if chunk.err != nil {
				return 0, chunk.err
			}
			c.remaining = chunk.data
		case <-c.done:
			return 0, net.ErrClosed
		}
	}
	n := copy(p, c.remaining)
	c.remaining = c.remaining[n:]
	return n, nil
}
func (c *delayedConn) Write(p []byte) (int, error) {
	chunk := delayedChunk{data: append([]byte(nil), p...), at: time.Now().Add(c.delay)}
	select {
	case c.outgoing <- chunk:
		return len(p), nil
	case <-c.done:
		return 0, net.ErrClosed
	}
}
func (c *delayedConn) Close() error {
	var err error
	c.once.Do(func() { close(c.done); err = c.Conn.Close() })
	return err
}
