package session

import (
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/bettercap/bettercap/core"
)

type Event struct {
	Tag  string      `json:"tag"`
	Time time.Time   `json:"time"`
	Data interface{} `json:"data"`
}

type LogMessage struct {
	Level   int
	Message string
}

func NewEvent(tag string, data interface{}) Event {
	return Event{
		Tag:  tag,
		Time: time.Now(),
		Data: data,
	}
}

func (e Event) Label() string {
	log := e.Data.(LogMessage)
	label := core.LogLabels[log.Level]
	color := core.LogColors[log.Level]
	return color + label + core.RESET
}

type EventPool struct {
	sync.Mutex

	debug     bool
	silent    bool
	events    []Event
	listeners []chan Event
}

func NewEventPool(debug bool, silent bool) *EventPool {
	return &EventPool{
		debug:     debug,
		silent:    silent,
		events:    make([]Event, 0),
		listeners: make([]chan Event, 0),
	}
}

func (p *EventPool) Listen() <-chan Event {
	p.Lock()
	defer p.Unlock()
	l := make(chan Event)

	// make sure, without blocking, the new listener
	// will receive all the queued events
	go func() {
		for i := len(p.events) - 1; i >= 0; i-- {
			l <- p.events[i]
		}
	}()

	p.listeners = append(p.listeners, l)
	return l
}

func (p *EventPool) Unlisten(listener <-chan Event) {
	p.Lock()
	defer p.Unlock()

	for i, l := range p.listeners {
		if l == listener {
			close(l)
			p.listeners = append(p.listeners[:i], p.listeners[i+1:]...)
			return
		}
	}
}

func (p *EventPool) SetSilent(s bool) {
	p.Lock()
	defer p.Unlock()
	p.silent = s
}

func (p *EventPool) SetDebug(d bool) {
	p.Lock()
	defer p.Unlock()
	p.debug = d
}

func (p *EventPool) Add(tag string, data interface{}) {
	p.Lock()
	defer p.Unlock()

	e := NewEvent(tag, data)
	p.events = append([]Event{e}, p.events...)

	// broadcast the event to every listener
	for _, l := range p.listeners {
		l <- e
	}
}

func (p *EventPool) Log(level int, format string, args ...interface{}) {
	if level == core.DEBUG && !p.debug {
		return
	} else if level < core.ERROR && p.silent {
		return
	}

	message := fmt.Sprintf(format, args...)

	p.Add("sys.log", LogMessage{
		level,
		message,
	})

	if level == core.FATAL {
		fmt.Fprintf(os.Stderr, "%s\n", message)
		os.Exit(1)
	}
}

func (p *EventPool) Clear() {
	p.Lock()
	defer p.Unlock()
	p.events = make([]Event, 0)
}

func (p *EventPool) Sorted() []Event {
	p.Lock()
	defer p.Unlock()

	sort.Slice(p.events, func(i, j int) bool {
		return p.events[i].Time.Before(p.events[j].Time)
	})

	return p.events
}
