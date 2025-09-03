package node

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"os"

	"github.com/luist18/gossip-glomers/pkg/id"
	"github.com/luist18/gossip-glomers/pkg/protocol"
)

type MessageHandler func(protocol.Message) error

type Node struct {
	id  string
	ids []string

	idGenerator id.SnowflakeGenerator

	handlers map[string]MessageHandler

	broadcastQueue []int64
}

func New() *Node {
	return &Node{
		handlers:       make(map[string]MessageHandler),
		idGenerator:    *id.NewSnowflakeGenerator(uint16(os.Getpid())),
		broadcastQueue: make([]int64, 0),
	}
}

func (n *Node) Handle(msgType string, handler MessageHandler) {
	n.handlers[msgType] = handler
}

func (n *Node) Start() {
	// fix: prevent overriding init handler and make it sure its is only called once
	// maybe add a flag to check if init was called
	n.handlers["init"] = n.init

	decoder := jsontext.NewDecoder(os.Stdin)
	for {
		msg := protocol.Message{}
		err := json.UnmarshalDecode(decoder, &msg)
		if err != nil {
			decoder.Reset(os.Stdin)
			fmt.Fprintf(os.Stderr, "failed to decode message: %v\n", err)
			continue
		}

		handler, ok := n.handlers[msg.Body.Type]
		if !ok {
			fmt.Fprintf(os.Stderr, "message ignored, no handler for `%s`\n", msg.Body.Type)
			continue
		}

		err = handler(msg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to handle message: %v\n", err)
		}
	}
}

func (n *Node) Reply(ask protocol.Message, body protocol.Body) error {
	encoder := jsontext.NewEncoder(os.Stdout)

	reply := ask
	reply.Dest = ask.Src
	reply.Src = ask.Dest

	reply.Body = body

	if ask.Body.MsgId != nil {
		reply.Body.InReplyTo = ask.Body.MsgId
	}

	// generate id

	return json.MarshalEncode(encoder, reply)
}

func (n *Node) init(msg protocol.Message) error {
	id, ok := msg.Body.Properties["node_id"].(string)
	if !ok {
		fmt.Fprintf(os.Stderr, "invalid init message: missing node_id\n")
		os.Exit(1)
	}

	idsRaw, ok := msg.Body.Properties["node_ids"].([]any)
	if !ok {
		fmt.Fprintf(os.Stderr, "invalid init message: missing node_ids\n")
		os.Exit(1)
	}

	ids := make([]string, len(idsRaw))
	for i, v := range idsRaw {
		idStr, ok := v.(string)
		if !ok {
			fmt.Fprintf(os.Stderr, "invalid init message: node_ids contains non-string value\n")
			os.Exit(1)
		}
		ids[i] = idStr
	}

	n.id = id
	n.ids = ids

	// in_reply_to is set in Reply
	err := n.Reply(msg, protocol.Body{
		Type: "init_ok",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to reply to init message: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "node initialized with id %s\n", n.id)
	return nil
}

func (n *Node) GenerateID() (int64, error) {
	return n.idGenerator.Generate()
}

func (n *Node) AddToBroadcastQueue(msg int64) {
	n.broadcastQueue = append(n.broadcastQueue, msg)
}

func (n *Node) ReadBroadcastQueue() []int64 {
	return n.broadcastQueue
}
