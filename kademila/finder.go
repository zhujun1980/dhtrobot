package kademila

import (
	"context"
	"net"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

type Finder struct {
	Chan      chan *Message
	ctx       context.Context
	routing   *table
	Working   atomic.Value
	Status    atomic.Value
	bootstrap []Node
}

func NewFinder(routing *table, ctx context.Context) (*Finder, error) {
	c, _ := FromContext(ctx)
	f := new(Finder)
	f.routing = routing
	f.ctx = ctx
	f.Chan = make(chan *Message)
	for _, host := range BOOTSTRAP {
		raddr, err := net.ResolveUDPAddr("udp", host)
		if err != nil {
			c.Log.WithFields(logrus.Fields{
				"err": err,
			}).Error("Resolve DNS error")
			continue
		}
		f.bootstrap = append(f.bootstrap, Node{Addr: raddr})
		c.Log.WithFields(logrus.Fields{
			"Host": host,
			"IP":   raddr.IP,
			"Port": raddr.Port,
		}).Info("Bootstrap from")
	}
	return f, nil
}

func (f *Finder) Forward(m *Message) {
	if f.Working.Load() == true && f.Status.Load() != Finished {
		f.Chan <- m
	}
}

func (f *Finder) Check() {
	if f.Working.Load() == true {
		if f.Status.Load() == Suspend {
			f.Status.Store(Finished)
		}
	} else {
		f.CheckTable()
	}
}

func (f *Finder) CheckTable() {
	if f.Working.Load() == true {
		return
	}
	f.Working.Store(true)
	go func() {
		c, _ := FromContext(f.ctx)
		now := time.Now()
		defer f.Working.Store(false)
		for i, b := range f.routing.buckets {
			diff := now.Sub(b.lastUpdated)
			if len(b.nodes) == 0 || diff.Minutes() >= BucketLastChangedTimeLimit {
				c.Log.Infof("Begin refresh bucket #%d [%x, %x)", i, b.min.Bytes(), b.max.Bytes())
				queriedNodes := make([]Node, len(f.bootstrap))
				copy(queriedNodes, f.bootstrap)
				for i := range b.nodes {
					queriedNodes = append(queriedNodes, b.nodes[i])
				}
				b.nodes = make([]Node, 0, K)
				f.FindNodes(b.generateRandomID(), queriedNodes)
				return
			} else {
				for i := range b.nodes {
					ndiff := now.Sub(b.nodes[i].LastSeen)
					if ndiff.Minutes() >= BucketLastChangedTimeLimit {
						b.nodes[i].Status = QUESTIONABLE
					}
				}
			}
		}
	}()
}

func (f *Finder) Bootstrap(target NodeID) {
	if f.Working.Load() == true {
		return
	}
	f.Working.Store(true)
	go func() {
		defer f.Working.Store(false)
		f.FindNodes(target, f.bootstrap)
	}()
}

func (f *Finder) FindNodes(target NodeID, queriedNodes []Node) error {
	c, _ := FromContext(f.ctx)
	allnodes := make(map[string]*Node)
	f.Status.Store(Running)
	for i := range queriedNodes {
		m := KRPCNewFindNode(c.Local.ID, target)
		m.N = queriedNodes[i]
		c.Outgoing <- m
		if len(queriedNodes[i].ID) > 0 {
			allnodes[queriedNodes[i].ID.String()] = &queriedNodes[i]
		}
	}
	minDistance := MaxBitsLength
	cond := true
	begin := time.Now()
	unchanged := 0

	c.Log.Infof("FindNode: %s", target.HexString())
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
			c.Log.Debugf("Got %d new nodes, total nodes %d, distance %d, unchanged %d", len(response.Nodes), len(allnodes), minDistance, unchanged)
			unchanged++
			node, ok := allnodes[response.ID]
			if ok {
				//c.Log.Infof("%s, v=%s", node, formatVersion(msg.V))
				if validateClient(msg.V) {
					node.Status = GOOD
				}
				distance := Distance(target, node.ID)
				if distance < minDistance {
					c.Log.WithFields(logrus.Fields{
						"prev": minDistance,
						"new":  distance,
					}).Debugf("Got closer node %s, current %s", node.ID.HexString(), target.HexString())
					minDistance = distance
					unchanged = 0
				}
			}
			if f.Status.Load() == Running && (unchanged >= MaxUnchangedCount) {
				f.Status.Store(Suspend)
				c.Log.Infof("FindNode finished, exceeds max count %d", MaxUnchangedCount)
			} else {
				for idx := range response.Nodes {
					if response.Nodes[idx].Port() <= 0 || net.IP(response.Nodes[idx].IP()).IsUnspecified() {
						continue
					}
					_, ok = allnodes[response.Nodes[idx].ID.String()]
					if !ok {
						allnodes[response.Nodes[idx].ID.String()] = &response.Nodes[idx]
						m := KRPCNewFindNode(c.Local.ID, target)
						m.N = response.Nodes[idx]
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
			f.routing.addNode(v)
		}
	}
	c.Log.Info(f.routing)
	c.Log.Infof("FindNode finished, got %d nodes, %d good nodes", len(allnodes), good)
	return nil
}
