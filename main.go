package main

import (
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

	node.Start()
}
