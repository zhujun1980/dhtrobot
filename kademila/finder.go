package kademila

import (
	"context"
	"net"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

type Finder struct {
	Chan    chan *Message
	ctx     context.Context
	routing *Routing
	Working atomic.Value
	Status  atomic.Value
}

func NewFinder(routing *Routing, ctx context.Context) (*Finder, error) {
	f := new(Finder)
	f.routing = routing
	f.ctx = ctx
	f.Chan = make(chan *Message)
	return f, nil
}

func (f *Finder) Forward(m *Message) {
	if f.Working.Load() == true && f.Status.Load() != Finished {
		f.Chan <- m
	}
}

func (f *Finder) Bootstrap() {
	c, _ := FromContext(f.ctx)

	var startNodes []Node
	for _, host := range BOOTSTRAP {
		raddr, err := net.ResolveUDPAddr("udp", host)
		if err != nil {
			c.Log.WithFields(logrus.Fields{
				"err": err,
			}).Error("Resolve DNS error")
			continue
		}
		startNodes = append(startNodes, Node{Addr: raddr})
		c.Log.WithFields(logrus.Fields{
			"Host": host,
			"IP":   raddr.IP,
			"Port": raddr.Port,
		}).Info("Bootstrap from")
	}
	f.FindNodes(c.Local.ID, startNodes)
}

func (f *Finder) FindNodes(target NodeID, queriedNodes []Node) error {
	c, _ := FromContext(f.ctx)

	f.Working.Store(true)
	defer f.Working.Store(false)
	f.Status.Store(Running)

	for _, v := range queriedNodes {
		m := KRPCNewFindNode(c.Local.ID, target)
		m.N = v
		c.Outgoing <- m
	}
	allnodes := make(map[string]*Node)
	minDistance := MaxBitsLength
	cond := true
	begin := time.Now()
	unchanged := 0
	for cond {
		if f.Status.Load() == Finished {
			break
		}
		select {
		case msg := <-f.Chan:
			if msg.Y != "r" {
				break
			}
			response, ok := msg.A.(*FindNodeResponse)
			if !ok {
				c.Log.WithFields(logrus.Fields{
					"msg": msg,
				}).Errorf("Incorrect msg, wants FindNode, got %s", msg.Q)
				break
			}
			c.Log.Infof("Got %d new nodes, total nodes %d, distance %d, unchanged %d", len(response.Nodes), len(allnodes), minDistance, unchanged)
			unchanged++
			node, ok := allnodes[response.ID]
			if ok {
				node.Status = GOOD
				distance := Distance(target, node.ID)
				if distance < minDistance {
					c.Log.WithFields(logrus.Fields{
						"prev": minDistance,
						"new":  distance,
					}).Infof("Got closer node %s, current %s", node.ID.HexString(), target.HexString())
					minDistance = distance
					unchanged = 0
				}
			}
			if f.Status.Load() == Running && (unchanged >= MaxUnchangedCount) {
				f.Status.Store(Suspend)
				c.Log.Infof("FindNode finished, exceeds max count %d", MaxUnchangedCount)
			} else {
				for _, v := range response.Nodes {
					if v.Port() <= 0 && net.IP(v.IP()).IsUnspecified() {
						continue
					}
					_, ok = allnodes[v.ID.String()]
					if !ok {
						allnodes[v.ID.String()] = v
						m := KRPCNewFindNode(c.Local.ID, target)
						m.N = *v
						c.Outgoing <- m
					}
				}
			}
			cond = false
			for _, v := range allnodes {
				if v.Status == INIT {
					cond = true
					break
				}
			}

		case <-time.After(time.Second):
			c.Log.Debug("Find node timeout")
			now := time.Now()
			diff := now.Sub(begin)
			if f.Status.Load() == Running && diff.Seconds() >= FindNodeTimeLimit {
				f.Status.Store(Suspend)
				c.Log.Infof("FindNode timeout, exceeds %d seconds", FindNodeTimeLimit)
			}

		case <-f.ctx.Done():
			c.Log.Error("FindNode canceled, ", f.ctx.Err())
			return nil
		}
	}

	good := 0
	for _, v := range allnodes {
		if v.Status == GOOD {
			good++
			f.routing.AddNode(v)
		}
	}
	c.Log.Infof("FindNode, got %d nodes, %d good nodes", len(allnodes), good)
	c.Log.Info(f.routing)
	return nil
}
