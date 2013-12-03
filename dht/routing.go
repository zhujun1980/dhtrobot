package dht

import (
	"container/list"
	"fmt"
	"time"
)

type Bucket struct {
	Min, Max   int
	Nodes      *list.List
	LastUpdate time.Time
}

func NewBucket(min, max int) *Bucket {
	b := new(Bucket)
	b.Min = min
	b.Max = max
	b.Nodes = list.New()
	b.LastUpdate = time.Now()
	return b
}

func (bucket *Bucket) Touch() {
	bucket.LastUpdate = time.Now()
}

func (bucket *Bucket) Len() int {
	return bucket.Nodes.Len()
}

func (bucket *Bucket) Add(n *NodeInfo) {
	bucket.Nodes.PushBack(n)
	bucket.Touch()
}

func (bucket *Bucket) Exists(node *NodeInfo) (*list.Element, bool) {
	l := bucket.Nodes
	for e := l.Front(); e != nil; e = e.Next() {
		ni := e.Value.(*NodeInfo)
		if ni.ID.CompareTo(node.ID) == 0 {
			return e, true
		}
	}
	return nil, false
}

func (bucket *Bucket) Copy(result *[]*NodeInfo, maxsize int) int {
	nw := 0
	l := bucket.Nodes
	for e := l.Back(); e != nil; e = e.Prev() {
		ni := e.Value.(*NodeInfo)
		if ni.Status == GOOD {
			*result = append(*result, ni)
			nw++
			if len(*result) == maxsize {
				break
			}
		}
	}
	return nw
}

func (bucket *Bucket) Print(own *NodeInfo) {
	l := bucket.Nodes
	for e := l.Front(); e != nil; e = e.Next() {
		ni := e.Value.(*NodeInfo)
		fmt.Println("\t\t", ni, "\td:", Distance(own.ID, ni.ID))
	}
}

type Table map[int]*Bucket

type Routing struct {
	ownNode *Node
	table   Table
}

func NewRouting(ownNode *Node) *Routing {
	routing := new(Routing)
	routing.ownNode = ownNode
	b := NewBucket(0, 160)
	routing.table = make(Table)
	routing.table[0] = b
	return routing
}

func (routing *Routing) Print() {
	tab := routing.table
	for i := 0; i < 160; i++ {
		if v, ok := tab[i]; ok {
			fmt.Println(v.Min, "-", v.Max, ":")
			v.Print(&routing.ownNode.Info)
		}
	}
}

func (routing *Routing) UpNode(node *NodeInfo) {
	if node.ID == nil || len(node.ID) == 0 {
		return
	}
	bucket, _ := routing.findBucket(node)
	if elem, ok := bucket.Exists(node); ok {
		n := elem.Value.(*NodeInfo)
		n.Status = GOOD
		routing.ownNode.Log.Printf("UpNode %s", n.ID.HexString())
		bucket.Nodes.MoveToBack(elem)
	}
}

func (routing *Routing) DownNode(node *NodeInfo) {
	if node.ID == nil || len(node.ID) == 0 {
		return
	}
	bucket, _ := routing.findBucket(node)
	if elem, ok := bucket.Exists(node); ok {
		n := elem.Value.(*NodeInfo)
		n.Status += 1
		routing.ownNode.Log.Printf("DownNode %s %d", n.ID.HexString(), n.Status)
		if n.Status == BAD {
			routing.ownNode.Log.Printf("%s become bad", n.ID.HexString())
			bucket.Nodes.Remove(elem)
		}
	}
}

func (routing *Routing) FindNode(other *NodeInfo, num int) []*NodeInfo {
	var result []*NodeInfo

	bucket, _ := routing.findBucket(other)
	p := bucket.Min
	n := bucket.Max

	for p >= 0 || n < 160 {
		if b, ok := routing.table[p]; ok {
			b.Copy(&result, num)
			if len(result) == num {
				break
			}
		}
		if b, ok := routing.table[n]; ok {
			b.Copy(&result, num)
			if len(result) == num {
				break
			}
		}
		p--
		n++
	}

	if len(result) > 0 {
		routing.ownNode.Log.Printf("Find nodes from routing, %s",
			NodesInfosToString(result))
	}
	return result
}

func (routing *Routing) InsertNode(other *NodeInfo) {
	if routing.isMe(other) {
		return
	}

	bucket, idx := routing.findBucket(other)
	if elem, ok := bucket.Exists(other); ok {
		bucket.Nodes.MoveToBack(elem)
		return
	}
	if bucket.Len() < K {
		bucket.Add(other)
		return
	}

	//TODO send ping
	if idx == 0 {
		routing.splitBucket(bucket)
		routing.InsertNode(other)
	}
}

func (routing *Routing) isMe(other *NodeInfo) bool {
	return (routing.ownNode.Info.ID.CompareTo(other.ID) == 0)
}

func (routing *Routing) splitBucket(bucket *Bucket) {
	if bucket.Max-1 == 0 {
		return
	}

	b := NewBucket(bucket.Max-1, bucket.Max)
	bucket.Max = bucket.Max - 1

	l := bucket.Nodes
	newlst := list.New()
	for e := l.Front(); e != nil; e = e.Next() {
		ni := e.Value.(*NodeInfo)
		idx := routing.bucketIndex(ni)
		if idx == b.Min {
			b.Nodes.PushBack(ni)
		} else {
			newlst.PushBack(ni)
		}
	}
	bucket.Nodes = newlst
	routing.table[b.Min] = b
}

func (routing *Routing) bucketIndex(dst *NodeInfo) int {
	return BucketIndex(routing.ownNode.Info.ID, dst.ID)
}

func (routing *Routing) findBucket(dst *NodeInfo) (*Bucket, int) {
	idx := routing.bucketIndex(dst)
	b, ok := routing.table[idx]
	if ok {
		return b, idx
	}
	return routing.table[0], 0
}
