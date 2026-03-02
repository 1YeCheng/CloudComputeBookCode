package client

import (
	"encoding/json"
	"net"
	"sync"
	"time"

	"cs/internal/proto"
)

type NetBackend struct {
	conn       net.Conn
	enc        *json.Encoder
	dec        *json.Decoder
	mu         sync.Mutex
	events     chan Event
	closed     chan struct{}
	closeOnce  sync.Once
	closedOnce sync.Once
}

const heartbeatInterval = 5 * time.Second

func NewNetBackend() *NetBackend {
	return &NetBackend{
		events: make(chan Event, 32),
		closed: make(chan struct{}),
	}
}

func (b *NetBackend) Connect(addr, name string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}

	b.conn = conn
	b.enc = json.NewEncoder(conn)
	b.dec = json.NewDecoder(conn)

	if err := b.send(proto.Message{Type: proto.TypeJoin, Join: &proto.Join{Name: name}}); err != nil {
		_ = conn.Close()
		return err
	}

	go b.readLoop()
	go b.heartbeatLoop()
	return nil
}

func (b *NetBackend) Close() error {
	b.closedOnce.Do(func() {
		close(b.closed)
	})
	if b.conn != nil {
		return b.conn.Close()
	}
	return nil
}

func (b *NetBackend) Events() <-chan Event {
	return b.events
}

func (b *NetBackend) Move(dx, dy int) error {
	return b.send(proto.Message{Type: proto.TypeMove, Move: &proto.Move{DX: dx, DY: dy}})
}

func (b *NetBackend) Challenge(targetID int) error {
	return b.send(proto.Message{Type: proto.TypeChallenge, Challenge: &proto.Challenge{TargetID: targetID}})
}

func (b *NetBackend) RespondChallenge(requestID int, accept bool) error {
	return b.send(proto.Message{Type: proto.TypeChallengeResponse, ChallengeResponse: &proto.ChallengeResponse{RequestID: requestID, Accept: accept}})
}

func (b *NetBackend) send(msg proto.Message) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.enc == nil {
		return nil
	}
	return b.enc.Encode(msg)
}

func (b *NetBackend) readLoop() {
	for {
		select {
		case <-b.closed:
			b.closeEvents()
			return
		default:
		}

		var msg proto.Message
		if err := b.dec.Decode(&msg); err != nil {
			b.closeEvents()
			return
		}
		b.handleMessage(msg)
	}
}

func (b *NetBackend) closeEvents() {
	b.closeOnce.Do(func() {
		close(b.events)
	})
}

func (b *NetBackend) handleMessage(msg proto.Message) {
	switch msg.Type {
	case proto.TypeWelcome:
		b.events <- Event{Type: EventWelcome, Welcome: msg.Welcome}
	case proto.TypeState:
		b.events <- Event{Type: EventState, State: msg.State}
	case proto.TypeDeltaState:
		b.events <- Event{Type: EventDeltaState, DeltaState: msg.DeltaState}
	case proto.TypeInfo:
		if msg.Info != nil {
			b.events <- Event{Type: EventInfo, Info: msg.Info.Message}
		}
	case proto.TypeChallengeRequest:
		b.events <- Event{Type: EventChallengeRequest, ChallengeRequest: msg.ChallengeRequest}
	case proto.TypeChallengeResult:
		b.events <- Event{Type: EventChallengeResult, ChallengeResult: msg.ChallengeResult}
	}
}

func (b *NetBackend) heartbeatLoop() {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.closed:
			return
		case <-ticker.C:
			_ = b.send(proto.Message{Type: proto.TypeHeartbeat, Heartbeat: &proto.Heartbeat{AtUnix: time.Now().Unix()}})
		}
	}
}
