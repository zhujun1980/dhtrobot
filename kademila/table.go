package kademila

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"sort"
	"sync"
	"time"
)

type bucket struct {
	ctx         context.Context
	min         *big.Int
	max         *big.Int
	nodes       []Node
	lastUpdated time.Time
}

func newBucket(ctx context.Context, min, max *big.Int) *bucket {
	b := new(bucket)
	b.ctx = ctx
	b.min = min
	b.max = max
	b.nodes = make([]Node, 0, K)
	b.lastUpdated = time.Now()
	return b
}

func (b *bucket) generateRandomID() NodeID {
	d := big.NewInt(0).Sub(b.max, b.min)
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	z := big.NewInt(0).Add(b.min, d.Rand(random, d))
	return z.Bytes()
}

func (b *bucket) compare(val *big.Int) int {
	if val.Cmp(b.min) >= 0 && val.Cmp(b.max) < 0 {
		return 0
	} else if val.Cmp(b.min) < 0 {
		return -1
	} else {
		return 1
	}
}

func (b *bucket) len() int {
	return len(b.nodes)
}

func (b *bucket) getInsertPosition(newnode *big.Int) int {
	return sort.Search(len(b.nodes), func(i int) bool {
		return b.nodes[i].ID.Int().Cmp(newnode) >= 0
	})
}

func (b *bucket) addNode(newnode *Node) {
	b.lastUpdated = time.Now()
	i := b.getInsertPosition(newnode.ID.Int())
	if i < len(b.nodes) && b.nodes[i].ID.Int().Cmp(newnode.ID.Int()) == 0 {
		b.nodes[i].LastSeen = time.Now()
		b.nodes[i].Status = GOOD
		if b.nodes[i].Addr != newnode.Addr {
			b.nodes[i].Addr = newnode.Addr
		}
		return
	}
	newnode.LastSeen = time.Now()
	newnode.Status = GOOD
	if i < len(b.nodes) {
		b.nodes = append(b.nodes, Node{})
		copy(b.nodes[i+1:], b.nodes[i:])
		b.nodes[i] = *newnode
	} else {
		b.nodes = append(b.nodes, *newnode)
	}
}

func (b *bucket) deleteNode(node *Node) {
	i := b.getInsertPosition(node.ID.Int())
	if i < len(b.nodes) && b.nodes[i].ID.Int().Cmp(node.ID.Int()) == 0 {
		var newnodes []Node
		for j := range b.nodes {
			if j != i {
				newnodes = append(newnodes, b.nodes[j])
			}
		}
		b.nodes = newnodes
	}
}

func (b *bucket) split() *bucket {
	min := big.NewInt(0)
	min.Add(b.max, b.min)
	min.Rsh(min, 1)

	nb := new(bucket)
	nb.ctx = b.ctx
	nb.min = min
	nb.max = b.max
	b.max = min
	nb.lastUpdated = b.lastUpdated

	i := b.getInsertPosition(nb.min)
	if i < b.len() {
		nb.nodes = make([]Node, len(b.nodes[i:]), K)
		copy(nb.nodes, b.nodes[i:])
		b.nodes = b.nodes[0:i]
	} else {
		nb.nodes = make([]Node, 0, K)
	}
	return nb
}

func (b *bucket) String() string {
	c, _ := FromContext(b.ctx)
	buf := bytes.NewBuffer(nil)
	buf.WriteString(fmt.Sprintf("[%x, %x), ", b.min.Bytes(), b.max.Bytes()))
	for i, n := range b.nodes {
		buf.WriteString(fmt.Sprintf("<%d, ID=%s, Addr=%s, S=%s, D=%d>", i, n.ID.HexString(), n.Addr.String(), n.SStatus(), Distance(n.ID, c.Local.ID)))
		if i < b.len() {
			buf.WriteString("; ")
		}
	}
	return buf.String()
}

type table struct {
	ctx        context.Context
	lock       *sync.Mutex
	buckets    []*bucket
	finderLock *sync.Mutex
	finders    []*finder
}

func newTable(ctx context.Context) *table {
	t := new(table)
	t.ctx = ctx
	t.lock = new(sync.Mutex)
	t.finderLock = new(sync.Mutex)

	min := big.NewInt(0)
	max := big.NewInt(1)
	max.Lsh(max, MaxBitsLength)
	t.buckets = append(t.buckets, newBucket(ctx, min, max))

	for i := 0; i < FinderNum; i++ {
		t.finders = append(t.finders, newFinder(ctx, i))
	}
	return t
}

func (t *table) searchBucket(key *big.Int) int {
	return sort.Search(len(t.buckets), func(i int) bool {
		return t.buckets[i].max.Cmp(key) > 0
	})
}

func (t *table) addNode(newnode *Node) {
	c, _ := FromContext(t.ctx)

	t.lock.Lock()
	defer t.lock.Unlock()

	if newnode.ID.String() == c.Local.ID.String() {
		return
	}

	k := newnode.ID.Int()
	idx := t.searchBucket(k)
	bk := t.buckets[idx]
	for {
		if bk.len() < K {
			bk.addNode(newnode)
			break
		} else if bk.compare(c.Local.ID.Int()) == 0 {
			nbk := bk.split()
			t.buckets = append(t.buckets, nil)
			copy(t.buckets[idx+2:], t.buckets[idx+1:])
			t.buckets[idx+1] = nbk
			if nbk.compare(k) == 0 {
				idx = idx + 1
				bk = nbk
			}
		} else {
			// discard it
			break
		}
	}
}

func (t *table) deleteNode(node *Node) {
	t.lock.Lock()
	defer t.lock.Unlock()
	idx := t.searchBucket(node.ID.Int())
	t.buckets[idx].deleteNode(node)
}

func (t *table) findNode(target string) []Node {
	t.lock.Lock()
	defer t.lock.Unlock()
	i := big.NewInt(0)
	i.SetBytes([]byte(target))
	idx := t.searchBucket(i)
	ret := make([]Node, t.buckets[idx].len())
	copy(ret, t.buckets[idx].nodes)
	return ret
}

func (t *table) getFinder() *finder {
	for i := range t.finders {
		if t.finders[i].available() {
			return t.finders[i]
		}
	}
	return nil
}

func (t *table) goFindNodes(finder *finder, target NodeID, queriedNodes []Node) {
	c, _ := FromContext(t.ctx)
	finder.working.Store(true)
	go func() {
		defer finder.working.Store(false)
		allnodes := finder.findNodes(target, queriedNodes)
		good := 0
		for _, v := range allnodes {
			if v.Status == GOOD {
				good++
				t.addNode(v)
			}
		}
		c.Log.Info(t)
		c.Log.Infof("FindNode(#%d) finished, got %d nodes, %d good nodes, table size: %d", finder.idx, len(allnodes), good, t.len())
	}()
}

func (t *table) goPingNodes(finder *finder, queriedNodes []Node) {
	c, _ := FromContext(t.ctx)
	finder.working.Store(true)
	go func() {
		defer finder.working.Store(false)
		allnodes := finder.findNodes(c.Local.ID, queriedNodes)
		good := 0
		for _, v := range allnodes {
			if v.Status == GOOD {
				good++
				t.addNode(v)
			} else if v.Status == QUESTIONABLE {
				t.deleteNode(v)
			}
		}
		c.Log.Info(t)
		c.Log.Infof("PingNode(#%d) finished, total: %d, good %d nodes, table size: %d", finder.idx, len(queriedNodes), good, t.len())
	}()
}

func (t *table) bootstrap(target NodeID) {
	t.finderLock.Lock()
	defer t.finderLock.Unlock()

	c, _ := FromContext(t.ctx)
	finder := t.getFinder()
	if finder != nil {
		t.goFindNodes(finder, target, c.bootstrap)
	}
}

func (t *table) forward(m *Message) {
	if m.W >= 0 && m.W < len(t.finders) {
		t.finders[m.W].forward(m)
	}
}

func (t *table) check() {
	t.finderLock.Lock()
	defer t.finderLock.Unlock()

	c, _ := FromContext(t.ctx)
	for _, f := range t.finders {
		if f.working.Load() == true {
			if f.status.Load() == Suspend {
				f.status.Store(Finished)
			}
		}
	}

	finder := t.getFinder()
	if finder != nil {
		t.lock.Lock()
		defer t.lock.Unlock()

		now := time.Now()
		for i, b := range t.buckets {
			diff := now.Sub(b.lastUpdated)
			if b.len() == 0 || diff.Minutes() >= BucketLastChangedTimeLimit {
				c.Log.Infof("Begin refresh bucket #%d [%x, %x)", i, b.min.Bytes(), b.max.Bytes())
				b.lastUpdated = now
				queriedNodes := make([]Node, len(c.bootstrap))
				copy(queriedNodes, c.bootstrap)
				for j := range b.nodes {
					queriedNodes = append(queriedNodes, b.nodes[j])
				}
				t.goFindNodes(finder, b.generateRandomID(), queriedNodes)
				return
			}
		}
		var ret []Node
		for _, b := range t.buckets {
			for i := range b.nodes {
				ndiff := now.Sub(b.nodes[i].LastSeen)
				if ndiff.Seconds() >= NodeRefreshnessTimeLimit {
					b.nodes[i].Status = QUESTIONABLE
					b.nodes[i].LastSeen = now
					ret = append(ret, b.nodes[i])
				}
			}
		}
		if len(ret) > 0 {
			t.goPingNodes(finder, ret)
		}
	}
}

func (t *table) len() int {
	l := 0
	for i := range t.buckets {
		l = l + t.buckets[i].len()
	}
	return l
}

func (t *table) String() string {
	c, _ := FromContext(t.ctx)
	buf := bytes.NewBuffer(nil)
	buf.WriteString(fmt.Sprintf("Local: %s, table: \n", c.Local))
	for i, v := range t.buckets {
		buf.WriteString(fmt.Sprintf("[%d]: time=%s, len=%d %v\n",
			i, v.lastUpdated.Format(time.RFC822), v.len(), v.String()))
	}
	return buf.String()
}
