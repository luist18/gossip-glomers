package main

import (
	"fmt"

	"github.com/luist18/gossip-glomers/pkg/node"
	"github.com/luist18/gossip-glomers/pkg/protocol"
)

func main() {
	node := node.New()

	node.Handle("echo", func(msg protocol.Message) error {
		body := msg.Body

		body.Type = "echo_ok"

		return node.Reply(msg, body)
	})

	node.Handle("generate", func(msg protocol.Message) error {
		id, err := node.GenerateID()
		if err != nil {
			return err
		}

		body := protocol.Body{
			Type: "generate_ok",
			Properties: map[string]any{
				"id": id,
			},
		}

		return node.Reply(msg, body)
	})

	node.Handle("broadcast", func(msg protocol.Message) error {
		body := protocol.Body{Type: "broadcast_ok"}

		message, ok := msg.Body.Properties["message"].(float64)
		if !ok {
			return fmt.Errorf("invalid broadcast message: missing message")
		}

		node.AddToBroadcastQueue(int64(message))

		// ack with broadcast_ok
		return node.Reply(msg, body)
	})

	node.Handle("read", func(msg protocol.Message) error {
		body := protocol.Body{
			Type: "read_ok",
			Properties: map[string]any{
				"messages": node.ReadBroadcastQueue(),
			},
		}

		return node.Reply(msg, body)
	})

	node.Handle("topology", func(msg protocol.Message) error {
		body := protocol.Body{
			Type: "topology_ok",
		}

		return node.Reply(msg, body)
	})

	node.Start()
}
