package kademila

import (
	"context"
	"net"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

type finder struct {
	Chan    chan *Message
	ctx     context.Context
	working atomic.Value
	status  atomic.Value
	idx     int
}

func newFinder(ctx context.Context, idx int) *finder {
	f := new(finder)
	f.ctx = ctx
	f.Chan = make(chan *Message)
	f.working.Store(false)
	f.idx = idx
	return f
}

func (f *finder) available() bool {
	return f.working.Load() == false
}

func (f *finder) forward(m *Message) {
	if f.working.Load() == true && f.status.Load() != Finished {
		f.Chan <- m
	}
}

func (f *finder) findNodes(target NodeID, queriedNodes []Node) map[string]*Node {
	c, _ := FromContext(f.ctx)
	allnodes := make(map[string]*Node)
	f.status.Store(Running)
	for i := range queriedNodes {
		m := KRPCNewFindNode(c.Local.ID, target, f.idx)
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

	c.Log.Infof("FindNode(#%d): %s start", f.idx, target.HexString())
	for cond {
		if f.status.Load() == Finished {
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
				}).Errorf("FindNode(#%d): Incorrect msg, wants FindNodeResponse, got %s", f.idx, msg.Q)
				break
			}
			c.Log.Debugf("FindNode(#%d): Got %d new nodes, total nodes %d, distance %d, unchanged %d", f.idx, len(response.Nodes), len(allnodes), minDistance, unchanged)
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
					}).Debugf("FindNode(#%d): Got closer node %s, current %s", f.idx, node.ID.HexString(), target.HexString())
					minDistance = distance
					unchanged = 0
				}
			}
			if f.status.Load() == Running && (unchanged >= MaxUnchangedCount) {
				f.status.Store(Suspend)
				c.Log.Infof("FindNode(#%d) stopped, exceeds max count %d", f.idx, MaxUnchangedCount)
			} else {
				for idx := range response.Nodes {
					if response.Nodes[idx].Port() <= 0 || net.IP(response.Nodes[idx].IP()).IsUnspecified() {
						continue
					}
					_, ok = allnodes[response.Nodes[idx].ID.String()]
					if !ok {
						allnodes[response.Nodes[idx].ID.String()] = &response.Nodes[idx]
						m := KRPCNewFindNode(c.Local.ID, target, f.idx)
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
			now := time.Now()
			diff := now.Sub(begin)
			if f.status.Load() == Running && diff.Seconds() >= FindNodeTimeLimit {
				f.status.Store(Suspend)
				c.Log.Infof("FindNode(#%d) timeout, exceeds %d seconds", f.idx, FindNodeTimeLimit)
			}

		case <-f.ctx.Done():
			c.Log.Errorf("FindNode(#%d) canceled, %s", f.idx, f.ctx.Err())
			return nil
		}
	}
	return allnodes
}

func (f *finder) pingNodes(queriedNodes []Node) []Node {
	c, _ := FromContext(f.ctx)
	var arrayIdx = make(map[string]int)
	begin := time.Now()
	f.status.Store(Running)
	for i := range queriedNodes {
		queriedNodes[i].LastSeen = begin
		m := KRPCNewPing(c.Local.ID, f.idx)
		m.N = queriedNodes[i]
		c.Outgoing <- m
		arrayIdx[queriedNodes[i].ID.String()] = i
	}

	cond := true
	c.Log.Infof("PingNode(#%d) start, stale nodes %d", f.idx, len(queriedNodes))
	for cond {
		if f.status.Load() == Finished {
			break
		}
		select {
		case msg := <-f.Chan:
			if msg.Y != "r" {
				break
			}
			response, ok := msg.A.(*PingResponse)
			if !ok {
				c.Log.WithFields(logrus.Fields{
					"msg": msg,
				}).Errorf("PingNode(#%d): Incorrect msg, wants PingResponse, got %s", f.idx, msg.Q)
				break
			}
			idx, ok := arrayIdx[response.ID]
			if ok {
				queriedNodes[idx].Status = GOOD
			} else {
				c.Log.Warnf("PingNode(#%d): ID not found %s", f.idx, response.ID)
			}

		case <-time.After(time.Second):
			now := time.Now()
			diff := now.Sub(begin)
			if f.status.Load() == Running {
				if diff.Seconds() >= PingNodeTimeLimit {
					f.status.Store(Suspend)
					c.Log.Debugf("PingNode(#%d) timeout, exceeds %d seconds", f.idx, PingNodeTimeLimit)
				} else {
					resend := 0
					for i := range queriedNodes {
						diff2 := now.Sub(queriedNodes[i].LastSeen)
						if queriedNodes[i].Status != GOOD && diff2.Seconds() >= RequestTimeout {
							queriedNodes[i].LastSeen = now
							m := KRPCNewPing(c.Local.ID, f.idx)
							m.N = queriedNodes[i]
							c.Outgoing <- m
							resend++
						}
					}
					c.Log.Debugf("PingNode(#%d) resend %d", f.idx, resend)
				}
			}

		case <-f.ctx.Done():
			c.Log.Errorf("PingNode(#%d) canceled, %s", f.idx, f.ctx.Err())
			return nil
		}
	}
	return queriedNodes
}
