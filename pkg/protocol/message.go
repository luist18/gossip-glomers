package protocol

type Message struct {
	Src  string `json:"src"`
	Dest string `json:"dest"`
	Body Body   `json:"body"`
}

func NewMessage(src, dest string, body Body) *Message {
	return &Message{
		Src:  src,
		Dest: dest,
		Body: body,
	}
}

type Body struct {
	// A string identifying the type of message this is
	Type string `json:"type"`
	// A unique identifier for this message
	MsgId *uint32 `json:"msg_id,omitzero"`
	// If this message is a reply, the msg_id of the original message
	InReplyTo *uint32 `json:"in_reply_to,omitzero"`

	Properties map[string]any `json:",inline"`
}

func NewBody(msgType string, properties map[string]any) *Body {
	return &Body{
		Type:       msgType,
		Properties: properties,
	}
}
