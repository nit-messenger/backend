package ws

import "encoding/json"

type Event struct {
	Type    string          `json:"type"`              // type of event (e.g. "new_message", "typing")
	Payload json.RawMessage `json:"payload,omitempty"` // event-specific details
}

type TypingPayload struct {
	ConversationID string `json:"conversation_id"`
	UserID         string `json:"user_id"`
	Username       string `json:"username"`
	IsTyping       bool   `json:"is_typing"`
}

type PresencePayload struct {
	UserID string `json:"user_id"`
	Status string `json:"status"` // online/offline/away
}

type ReadReceiptPayload struct {
	ConversationID string `json:"conversation_id"`
	UserID         string `json:"user_id"`
	LastReadAt     string `json:"last_read_at"`
}

type CallSignalPayload struct {
	CallID string          `json:"call_id"`
	Sender string          `json:"sender"`
	Target string          `json:"target,omitempty"` // empty for broadcast in group
	Signal json.RawMessage `json:"signal"`           // SDP offer/answer/candidate
}

type CallJoinPayload struct {
	ConversationID string `json:"conversation_id"`
	SDP            string `json:"sdp"`
}

type CallAnswerPayload struct {
	ConversationID string `json:"conversation_id"`
	SDP            string `json:"sdp"`
}

type CallCandidatePayload struct {
	ConversationID string `json:"conversation_id"`
	Candidate      string `json:"candidate"` // JSON serialized candidate
}

