package hub

import (
	"encoding/json"
	"time"
)

// Envelope is the single frame format used on every transport (WS, SSE, poll).
type Envelope struct {
	Type string          `json:"type"`
	ID   string          `json:"id,omitempty"` // client-generated id for ack correlation
	Data json.RawMessage `json:"data,omitempty"`
	TS   int64           `json:"ts,omitempty"` // server unix millis
}

// Inbound visitor message types.
const (
	TypeConversationStart = "conversation.start"
	TypeMessageSend       = "message.send"
	TypeTypingStart       = "typing.start"
	TypeTypingStop        = "typing.stop"
	TypeSyncRequest       = "sync.request"
	TypeReadAck           = "read.ack"
	TypeCSATSubmit        = "csat.submit"
	TypePing              = "ping"
	TypePong              = "pong"
)

// Outbound visitor message types.
const (
	TypeConversationCreated  = "conversation.created"
	TypeMessageSent          = "message.sent"
	TypeMessageReceived      = "message.received"
	TypeAgentAssigned        = "agent.assigned"
	TypeAgentTyping          = "agent.typing"
	TypeAgentTypingStop      = "agent.typing.stop"
	TypeConversationResolved = "conversation.resolved"
	TypeConversationUpdated  = "conversation.updated"
	TypeSyncResponse         = "sync.response"
	TypeCSATRequest          = "csat.request"
	TypeError                = "error"
)

// Inbound agent message types.
const (
	TypeConversationAssign   = "conversation.assign"
	TypeConversationResolve  = "conversation.resolve"
	TypeConversationTransfer = "conversation.transfer"
	TypeConversationReopen   = "conversation.reopen"
	TypeNoteAdd              = "note.add"
	TypePresenceSet          = "presence.set"
)

// Outbound agent message types.
const (
	TypeConversationNew   = "conversation.new"
	TypeVisitorTyping     = "visitor.typing"
	TypeVisitorTypingStop = "visitor.typing.stop"
	TypeVisitorOnline     = "visitor.online"
	TypeVisitorOffline    = "visitor.offline"
	TypePresenceUpdate    = "presence.update"
	TypeStatsTick         = "stats.tick"
	TypeMessageAck        = "message.ack"
	TypeNoteAdded         = "note.added"
)

// NewEnvelope builds an envelope with a typed payload.
func NewEnvelope(typ string, data any) Envelope {
	e := Envelope{Type: typ}
	if data != nil {
		b, err := json.Marshal(data)
		if err == nil {
			e.Data = b
		}
	}
	return e
}

// WithTS stamps the envelope with the current server time.
func (e Envelope) WithTS(ts int64) Envelope {
	e.TS = ts
	return e
}

// ErrorData is the payload of an "error" envelope.
type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorEnvelope builds an error frame.
func ErrorEnvelope(code, message string) Envelope {
	return NewEnvelope(TypeError, ErrorData{Code: code, Message: message})
}

// Decode unmarshals the envelope payload into dst.
func (e Envelope) Decode(dst any) error {
	if len(e.Data) == 0 {
		return nil
	}
	return json.Unmarshal(e.Data, dst)
}

func nowMS() int64 { return time.Now().UnixMilli() }
