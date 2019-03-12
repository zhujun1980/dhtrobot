package kademila

import (
	"context"
	"net"
	"time"

	"github.com/sirupsen/logrus"
)

type Kademila struct {
	Chan    chan string
	ctx     context.Context
	routing *Routing
	finder  *Finder
	token   *TokenBuilder
}

func Restore(master chan string, id NodeID, routing []byte) *Kademila {
	k := new(Kademila)
	// k.Local.ID = id
	// k.master = master
	return k
}

func New(ctx context.Context, master chan string, logger *logrus.Logger) (*Kademila, error) {
	var err error

	k := new(Kademila)
	c := new(NodeContext)
	c.Log = logger
	c.Master = master
	c.Outgoing = make(chan *Message)
	c.Incoming = make(chan RawData)
	c.Conn, err = net.ListenPacket("udp", "")
	if err != nil {
		c.Log.Panic(err)
	}
	c.Local.ID = GenerateID()
	c.Local.Addr = c.Conn.LocalAddr().(*net.UDPAddr)
	c.Local.Status = GOOD

	k.ctx = NewContext(ctx, c)
	k.Chan = make(chan string)
	k.routing, err = NewRouting(k.ctx)
	if err != nil {
		return nil, err
	}
	k.finder, err = NewFinder(k.routing, k.ctx)
	if err != nil {
		return nil, err
	}
	k.token = newTokenBuilder()

	c.Log.WithFields(logrus.Fields{
		"ID":   c.Local.ID.HexString(),
		"Addr": c.Local.Addr.String(),
	}).Info("Node started success")

	go func() { k.mainLoop(true) }()
	go func() { k.incomingLoop() }()
	go func() { k.outgoingLoop() }()

	return k, nil
}

func (k *Kademila) mainLoop(bootstrap bool) {
	c, _ := FromContext(k.ctx)

	if bootstrap {
		go func() { k.finder.Bootstrap() }()
	}
	for {
		select {
		case msg := <-k.Chan:
			c.Log.WithFields(logrus.Fields{
				"msg": msg,
			}).Debug("Receive from master")

		case raw := <-c.Incoming:
			msg, err := KRPCDecode(&raw)
			if err != nil {
				c.Log.WithFields(logrus.Fields{
					"err": err,
				}).Error("Encode failed")
				break
			}
			c.Log.WithFields(logrus.Fields{
				"m": msg.String(),
			}).Debug("Encode success")
			k.processMessage(msg)

		case <-time.After(time.Second):
			//c.Log.Debug("Main Loop Timeout")
		}

		k.transition()
	}
}

func (k *Kademila) transition() {
	c, _ := FromContext(k.ctx)

	if k.finder.Working.Load() == true {
		if k.finder.Status.Load() == Suspend {
			k.finder.Status.Store(Finished)
			c.Log.Info("Finder job finished")
		}
	} else {
	}

	k.token.renewToken()
}

func (k *Kademila) processQuery(m *Message) error {
	c, _ := FromContext(k.ctx)

	switch m.Q {
	case "ping":
		out := KRPCNewPingResponse(m.T, c.Local.ID)
		out.N = m.N
		c.Outgoing <- out
	case "find_node":
		var out *Message
		q := m.A.(*FindNodeQuery)
		nodes, err := k.routing.FindNode(q.Target)
		if err != nil {
			out = KRPCNewError(m.T, m.Q, ServerError)
			c.Log.WithFields(logrus.Fields{
				"err": err,
			}).Error("FindNode failed")
		} else {
			out = KRPCNewFindNodeResponse(m.T, c.Local.ID, nodes)
		}
		out.N = m.N
		c.Outgoing <- out
	case "get_peers":
		var out *Message
		q := m.A.(*GetPeersQuery)
		nodes, err := k.routing.FindNode(q.InfoHash)
		if err != nil {
			out = KRPCNewError(m.T, m.Q, ServerError)
			c.Log.WithFields(logrus.Fields{
				"err": err,
			}).Error("FindNode failed")
		} else {
			out = KRPCNewGetPeersResponse(m.T, c.Local.ID, k.token.create(m.N.Addr.String()), nodes, []*Peer{})
		}
		out.N = m.N
		c.Outgoing <- out
	case "announce_peer":
	}
	return nil
}

func (k *Kademila) processResponse(m *Message) error {
	// c, _ := FromContext(k.ctx)
	switch m.Q {
	case "ping":
	case "find_node":
		k.finder.Forward(m)
	case "get_peers":
	case "announce_peer":
	}
	return nil
}

func (k *Kademila) processError(m *Message) error {
	return nil
}

func (k *Kademila) processMessage(m *Message) error {
	switch m.Y {
	case "q":
		return k.processQuery(m)
	case "r":
		return k.processResponse(m)
	case "e":
		return k.processError(m)
	}
	return nil
}

func (k *Kademila) writeMessage(m *Message, addr net.Addr) {
	c, _ := FromContext(k.ctx)

	encoded, err := KRPCEncode(m)
	if err != nil {
		c.Log.WithFields(logrus.Fields{
			"err": err,
		}).Fatal("Encode failed")
		return
	}
	var n int
	n, err = c.Conn.WriteTo([]byte(encoded), addr)
	if err != nil {
		c.Log.WithFields(logrus.Fields{
			"length": len(encoded),
			"wrote":  n,
			"err":    err,
		}).Fatal("Write failed")
		return
	}
	c.Log.WithFields(logrus.Fields{
		"length":      len(encoded),
		"wrote":       n,
		"destination": addr.String(),
		"m":           m,
	}).Debug("Packet written")
}

func (k *Kademila) outgoingLoop() {
	c, _ := FromContext(k.ctx)

	for {
		select {
		case msg := <-c.Outgoing:
			k.writeMessage(msg, msg.N.Addr)

		case <-time.After(time.Second):
			//c.Log.Debug("Outgoing Loop Timeout")
		}
	}
}

func (k *Kademila) incomingLoop() {
	c, _ := FromContext(k.ctx)

	data := make([]byte, MAXSIZE)
	for {
		n, addr, err := c.Conn.ReadFrom(data)
		if err != nil {
			c.Log.WithFields(logrus.Fields{
				"err": err,
			}).Error("Connection read failed")
			break
		}
		c.Log.WithFields(logrus.Fields{
			"bytes": n,
			"addr":  addr.String(),
		}).Debug("Packet received")
		c.Incoming <- RawData{addr, data[:n]}
	}
}

func (k *Kademila) GetPeers(infoHash string) {
}

func (k *Kademila) AnnouncePeers(impliedPort bool, infoHash string, port int, token string) {
}

func (k *Kademila) Close() {
}