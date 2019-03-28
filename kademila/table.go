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
	i := b.getInsertPosition(newnode.ID.Int())
	if i < len(b.nodes) && b.nodes[i].ID.Int().Cmp(newnode.ID.Int()) == 0 {
		b.nodes[i].LastSeen = time.Now()
		b.nodes[i].Status = GOOD
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
	b.lastUpdated = time.Now()
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
		buf.WriteString(fmt.Sprintf("<%d, ID=%s, Addr=%s, D=%d>", i, n.ID.HexString(), n.Addr.String(), Distance(n.ID, c.Local.ID)))
		if i < b.len() {
			buf.WriteString("; ")
		}
	}
	return buf.String()
}

type table struct {
	ctx     context.Context
	lock    *sync.Mutex
	buckets []*bucket
}

func newTable(ctx context.Context) *table {
	//c, _ := FromContext(ctx)

	t := new(table)
	t.ctx = ctx
	t.lock = new(sync.Mutex)

	min := big.NewInt(0)
	max := big.NewInt(1)
	max.Lsh(max, MaxBitsLength)
	t.buckets = append(t.buckets, newBucket(ctx, min, max))
	//t.addNode(&c.Local)
	return t
}

func (t *table) addNode(newnode *Node) {
	c, _ := FromContext(t.ctx)

	t.lock.Lock()
	defer t.lock.Unlock()

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

func (t *table) searchBucket(key *big.Int) int {
	i := sort.Search(len(t.buckets), func(i int) bool {
		return t.buckets[i].max.Cmp(key) > 0
	})
	return i
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
