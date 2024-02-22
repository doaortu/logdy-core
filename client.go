package main

import (
	"sync"
	"time"
)

var BULK_WINDOW_MS int64 = 100
var FLUSH_BUFFER_SIZE = 1000
var MAX_MESSAGE_COUNT = 100_000

type CursorStatus string

const CURSOR_STOPPED CursorStatus = "stopped"
const CURSOR_FOLLOWING CursorStatus = "following"

type Client struct {
	bufferOpMu sync.Mutex
	id         string
	done       chan struct{}
	ch         chan []Message
	buffer     []Message

	cursorStatus   CursorStatus
	cursorPosition string // last delivered message id
}

func (c *Client) handleMessage(m Message, force bool) {
	if !force && c.cursorStatus == CURSOR_STOPPED {
		logger.Debug("Client: Status stopped discarding message")
		return
	}
	c.buffer = append(c.buffer, m)
}

func (c *Client) flushBuffer() {
	if len(c.buffer) == 0 {
		return
	}

	for i := 0; i < len(c.buffer); i += FLUSH_BUFFER_SIZE {
		end := i + FLUSH_BUFFER_SIZE
		if end > len(c.buffer) {
			end = len(c.buffer)
		}

		batch := c.buffer[i:end]
		c.ch <- batch
	}
}

func (c *Client) clearBuffer() {
	c.buffer = []Message{}
}

func (c *Client) close() {
	c.done <- struct{}{}
}

func (c *Client) waitForBufferDrain() {
	for len(c.buffer) > 0 {
		time.Sleep(5 * time.Millisecond)
	}
}

// Messages are delivered in bulks to avoid
// ddossing the client (browser) with too many messages produced
// in a very short timespan
func (c *Client) startBufferFlushLoop() {
	for {
		time.Sleep(time.Millisecond * time.Duration(BULK_WINDOW_MS))
		select {
		case <-c.done:
			logger.Debug("Client: received done signal, quitting")
			defer close(c.done)
			defer close(c.ch)
			return
		default:

			if len(c.buffer) == 0 {
				continue
			}

			logger.WithField("count", len(c.buffer)).Debug("Client: Flushing buffer")
			c.cursorPosition = c.buffer[len(c.buffer)-1].Id
			c.bufferOpMu.Lock()

			c.flushBuffer()
			c.clearBuffer()

			c.bufferOpMu.Unlock()
		}

	}
}

func NewClient() *Client {
	c := &Client{
		bufferOpMu:     sync.Mutex{},
		done:           make(chan struct{}),
		ch:             make(chan []Message, BULK_WINDOW_MS*25),
		cursorStatus:   CURSOR_STOPPED,
		cursorPosition: "",
		id:             RandStringRunes(6),
	}

	go c.startBufferFlushLoop()

	return c
}

type Clients struct {
	started            bool
	mu                 sync.Mutex
	mainChan           <-chan Message
	clients            map[string]*Client
	buffer             []Message
	currentlyConnected int
	stats              Stats
}

func NewClients(msgs <-chan Message) *Clients {
	cls := &Clients{
		mu:                 sync.Mutex{},
		mainChan:           msgs,
		clients:            map[string]*Client{},
		currentlyConnected: 0,
		buffer:             []Message{},
		stats: Stats{
			Count: 0,
		},
	}

	go cls.Start()

	return cls
}

func (c *Clients) Load(clientId string, start int, count int, includeStart bool) {
	c.PauseFollowing(clientId)
	cl := c.clients[clientId]
	cl.waitForBufferDrain()

	cl.bufferOpMu.Lock()
	defer cl.bufferOpMu.Unlock()

	seen := false
	sent := 0
	for i, msg := range c.buffer {
		if i+1 == start {
			seen = true
			if !includeStart {
				continue
			}
		}

		if !seen {
			continue
		}

		sent++
		cl.handleMessage(msg, true)

		if count > 0 && sent >= count {
			break
		}
	}
	cl.flushBuffer()

}

func (c *Clients) PeekLog(idxs []int) []Message {
	msgs := []Message{}

	for _, idx := range idxs {
		if len(c.buffer)-1 < idx {
			continue
		}
		msgs = append(msgs, c.buffer[idx])
	}

	return msgs
}

type Stats struct {
	Count          int
	FirstMessageAt time.Time
	LastMessageAt  time.Time
}

func (c *Clients) Stats() Stats {
	return c.stats
}

func (c *Clients) ResumeFollowing(clientId string, sinceCursor bool) {
	//pump back the items until last element seen

	c.clients[clientId].bufferOpMu.Lock()
	if sinceCursor {
		seen := false
		for _, msg := range c.buffer {
			if msg.Id == c.clients[clientId].cursorPosition {
				seen = true
				continue
			}

			if !seen {
				continue
			}

			c.clients[clientId].handleMessage(msg, true)
		}
	}
	c.clients[clientId].flushBuffer()
	c.clients[clientId].cursorStatus = CURSOR_FOLLOWING
	c.clients[clientId].bufferOpMu.Unlock()
}

func (c *Clients) PauseFollowing(clientId string) {
	c.clients[clientId].cursorStatus = CURSOR_STOPPED
	c.clients[clientId].waitForBufferDrain()
}

// starts a delivery channel to all clients
func (c *Clients) Start() {
	if c.started {
		logger.Debug("Clients delivery loop already started")
		return
	}

	c.started = true
	for {
		msg := <-c.mainChan
		if c.stats.FirstMessageAt.IsZero() {
			c.stats.FirstMessageAt = time.Now()
		}
		c.buffer = append(c.buffer, msg)
		c.stats.Count++
		c.stats.LastMessageAt = time.Now()

		for _, ch := range c.clients {
			ch.bufferOpMu.Lock()
			ch.handleMessage(msg, false)
			ch.bufferOpMu.Unlock()
		}
	}
}

func (c *Clients) Join(tailLen int) *Client {
	cl := NewClient()
	c.clients[cl.id] = cl
	c.currentlyConnected++

	// deliver last N messages from a buffer upon connection
	idx := 0
	if len(c.buffer) > tailLen {
		idx = len(c.buffer) - tailLen
	}

	for _, msg := range c.buffer[idx:] {
		cl.handleMessage(msg, true)
	}
	c.clients[cl.id].cursorStatus = CURSOR_FOLLOWING

	return c.clients[cl.id]
}

func (c *Clients) Close(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.clients[id]; !ok {
		return
	}

	cl := c.clients[id]
	cl.close()
	delete(c.clients, id)
	c.currentlyConnected--
}
