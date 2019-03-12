package kademila

import (
	"bytes"
	"container/list"
	"context"
	"fmt"
	"sync"
	"time"
)

type Bucket struct {
	ctx        context.Context
	Nodes      *list.List
	LastUpdate time.Time
	lock       *sync.Mutex
	secondary  *list.List
}

func NewBucket(ctx context.Context) *Bucket {
	b := new(Bucket)
	b.ctx = ctx
	b.Nodes = list.New()
	b.LastUpdate = time.Now()
	b.secondary = list.New()
	b.lock = new(sync.Mutex)
	return b
}

func (b *Bucket) addNode(node *Node) {
	b.lock.Lock()
	defer b.lock.Unlock()

	if b.Nodes.Len() < K {
		b.Nodes.PushFront(node)
		b.LastUpdate = time.Now()
	} else {
		//discard
	}
}

func (b *Bucket) findNodes(target NodeID) ([]*Node, error) {
	var ret []*Node

	b.lock.Lock()
	defer b.lock.Unlock()

	for e := b.Nodes.Front(); e != nil; e = e.Next() {
		n := e.Value.(*Node)
		if 0 == bytes.Compare(n.ID, target) {
			ret = append(ret, n.Clone())
			return ret, nil
		}
	}
	for e := b.Nodes.Front(); e != nil; e = e.Next() {
		n := e.Value.(*Node)
		ret = append(ret, n.Clone())
	}
	return ret, nil
}

func (b *Bucket) String() string {
	c, _ := FromContext(b.ctx)
	buf := bytes.NewBuffer(nil)
	for i, e := 0, b.Nodes.Front(); e != nil; e = e.Next() {
		n := e.Value.(*Node)
		buf.WriteString(fmt.Sprintf("<%d, ID=%s, Addr=%s, D=%d>", i, n.ID.HexString(), n.Addr.String(), Distance(n.ID, c.Local.ID)))
		if e.Next() != nil {
			buf.WriteString("; ")
		}
		i++
	}
	return buf.String()
}

type Routing struct {
	ctx     context.Context
	Buckets []*Bucket
	Chan    chan *Message
}

func NewRouting(ctx context.Context) (*Routing, error) {
	r := new(Routing)
	r.ctx = ctx
	r.Buckets = make([]*Bucket, MaxBitsLength)
	for i := range r.Buckets {
		r.Buckets[i] = NewBucket(ctx)
	}
	r.Chan = make(chan *Message)
	return r, nil
}

func (r *Routing) String() string {
	buf := bytes.NewBuffer(nil)
	for i, v := range r.Buckets {
		buf.WriteString(fmt.Sprintf("[%d]: time=%s, len=%d %v\n",
			i, v.LastUpdate.Format(time.RFC822), v.Nodes.Len(), v.String()))
	}
	return buf.String()
}

func (r *Routing) GetBucket(d int) *Bucket {
	c, _ := FromContext(r.ctx)
	i := d - 1
	if i < 0 || i >= 160 {
		c.Log.Panicf("Distance value should be in [1, 160], got %d", d)
		return nil
	}
	return r.Buckets[i]
}

func (r *Routing) AddNode(newnode *Node) {
	c, _ := FromContext(r.ctx)
	d := Distance(newnode.ID, c.Local.ID)
	if d == 0 {
		c.Log.Warnf("Same node, local: %s, new: %s", c.Local.String(), newnode.String())
		return
	}
	bucket := r.GetBucket(d)
	bucket.addNode(newnode)
}

func (r *Routing) FindNode(t string) ([]*Node, error) {
	target := []byte(t)
	c, _ := FromContext(r.ctx)
	d := Distance(target, c.Local.ID)
	if d == 0 {
		ret := make([]*Node, 1)
		ret[0] = (&c.Local).Clone()
		return ret, nil
	}
	bucket := r.GetBucket(d)
	return bucket.findNodes(target)
}
