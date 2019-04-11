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
	routing *table
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
	k.routing = newTable(k.ctx)
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
		k.finder.Bootstrap(c.Local.ID)
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
				}).Error("Decode failed")
				break
			}
			k.processMessage(msg)

		case <-time.After(time.Second):
			//c.Log.Debug("Main Loop Timeout")
		}

		k.transition()
	}
}

func (k *Kademila) transition() {
	k.finder.Check()
	k.token.renewToken()
}

func (k *Kademila) processQuery(m *Message) error {
	var out *Message
	c, _ := FromContext(k.ctx)

	c.Log.WithFields(logrus.Fields{
		"m": m.String(),
	}).Info("Request received:")

	switch m.Q {
	case "ping":
		out = KRPCNewPingResponse(m.T, c.Local.ID)
	case "find_node":
		q := m.A.(*FindNodeQuery)
		nodes := k.routing.findNode(q.Target)
		out = KRPCNewFindNodeResponse(m.T, c.Local.ID, nodes)
	case "get_peers":
		q := m.A.(*GetPeersQuery)
		nodes := k.routing.findNode(q.InfoHash)
		out = KRPCNewGetPeersResponse(m.T, c.Local.ID, k.token.create(m.N.Addr.String()), nodes, []*Peer{})
	case "announce_peer":
	}

	if validateClient(m.V) {
		k.routing.addNode(&m.N)
	}
	out.N = m.N
	c.Outgoing <- out
	return nil
}

func (k *Kademila) processResponse(m *Message) error {
	c, _ := FromContext(k.ctx)

	c.Log.WithFields(logrus.Fields{
		"m": m.String(),
	}).Debug("Response received:")

	switch m.Q {
	case "ping":
	case "find_node":
		k.finder.Forward(m)
	case "get_peers":
	case "announce_peer":
	}

	if validateClient(m.V) {
		k.routing.addNode(&m.N)
	}
	return nil
}

func (k *Kademila) processError(m *Message) error {
	c, _ := FromContext(k.ctx)
	c.Log.WithFields(logrus.Fields{
		"m": m.String(),
	}).Warn("Error received:")
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
			if msg.N.Port() > 0 {
				k.writeMessage(msg, msg.N.Addr)
			}

		case <-time.After(time.Second):
			//c.Log.Debug("Outgoing Loop Timeout")
		}
	}
}

func (k *Kademila) incomingLoop() {
	c, _ := FromContext(k.ctx)

	buffer := make(map[string][]byte)
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

		if KRPCValidate(data[:n]) {
			newdat := make([]byte, n)
			copy(newdat, data[:n])
			c.Incoming <- RawData{addr, newdat}
		} else {
			key := addr.String()
			_, ok := buffer[key]
			if !ok {
				buffer[key] = make([]byte, n)
				copy(buffer[key], data[:n])
			} else {
				cur := len(buffer[key])
				newdat := make([]byte, cur+n)
				copy(newdat, buffer[key])
				copy(newdat[cur:], data[:n])
				if KRPCValidate(newdat) {
					c.Incoming <- RawData{addr, newdat}
					delete(buffer, key)
				} else {
					buffer[key] = newdat
				}
			}
		}
	}
}

func (k *Kademila) GetPeers(infoHash string) {
}

func (k *Kademila) AnnouncePeers(impliedPort bool, infoHash string, port int, token string) {
}

func (k *Kademila) Close() {
}
