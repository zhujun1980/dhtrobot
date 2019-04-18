package kademila

import (
	"context"
	"io"
	"net"

	"github.com/sirupsen/logrus"
)

type NodeContext struct {
	Local     Node
	Log       *logrus.Logger
	Conn      net.PacketConn
	Master    chan string
	Outgoing  chan *Message
	Incoming  chan RawData
	Writer    io.Writer
	bootstrap []Node
}

type key int

const contextKey key = 0

func newContext(ctx context.Context, master chan string, logger *logrus.Logger, writer io.Writer) context.Context {
	var err error

	c := new(NodeContext)
	c.Master = master
	c.Log = logger
	c.Writer = writer
	c.Outgoing = make(chan *Message)
	c.Incoming = make(chan RawData)
	c.Conn, err = net.ListenPacket("udp", "")
	if err != nil {
		c.Log.Panic(err)
	}
	c.Local.ID = GenerateID()
	c.Local.Addr = c.Conn.LocalAddr().(*net.UDPAddr)
	c.Local.Status = GOOD
	for _, host := range BOOTSTRAP {
		raddr, err := net.ResolveUDPAddr("udp", host)
		if err != nil {
			c.Log.WithFields(logrus.Fields{
				"err": err,
			}).Error("Resolve DNS error")
			continue
		}
		c.bootstrap = append(c.bootstrap, Node{Addr: raddr})
		c.Log.WithFields(logrus.Fields{
			"Host": host,
			"IP":   raddr.IP,
			"Port": raddr.Port,
		}).Info("Bootstrap from")
	}
	return context.WithValue(ctx, contextKey, c)
}

func FromContext(ctx context.Context) (*NodeContext, bool) {
	c, ok := ctx.Value(contextKey).(*NodeContext)
	return c, ok
}
