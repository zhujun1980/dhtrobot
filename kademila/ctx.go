package kademila

import (
	"context"
	"io"
	"net"

	"github.com/sirupsen/logrus"
)

type NodeContext struct {
	Local    Node
	Log      *logrus.Logger
	Conn     net.PacketConn
	Master   chan string
	Outgoing chan *Message
	Incoming chan RawData
	Writer   io.Writer
}

type key int

const contextKey key = 0

func NewContext(ctx context.Context, c *NodeContext) context.Context {
	return context.WithValue(ctx, contextKey, c)
}

func FromContext(ctx context.Context) (*NodeContext, bool) {
	c, ok := ctx.Value(contextKey).(*NodeContext)
	return c, ok
}
