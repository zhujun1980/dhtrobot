package dht

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math/big"
	"time"
	"math/rand"
)

type Bucket struct {
	Min, Max   *big.Int
	Nodes      []*NodeInfo
	LastUpdate time.Time
}

func NewBucket(min, max *big.Int) *Bucket {
	b := new(Bucket)
	b.Min = min
	b.Max = max
	b.Nodes = nil
	b.LastUpdate = time.Now()
	return b
}

func (bucket *Bucket) Touch() {
	bucket.LastUpdate = time.Now()
}

func (bucket *Bucket) Len() int {
	return len(bucket.Nodes)
}

func (bucket *Bucket) Add(n *NodeInfo) {
	bucket.Nodes = append(bucket.Nodes, n)
	bucket.Touch()
}

func (bucket *Bucket) UpdateIfExists(node *NodeInfo) bool {
	for idx, ni := range bucket.Nodes {
		if ni.ID.CompareTo(node.ID) == 0 {
			bucket.Nodes[idx] = node
			return true
		}
	}
	return false
}

func (bucket *Bucket) Copy(result *[]*NodeInfo, maxsize int) int {
	nw := 0
	for _, ni := range bucket.Nodes {
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

func (bucket *Bucket) RandID() Identifier {
	d := bisub(bucket.Max, bucket.Min)
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	z := biadd(bucket.Min, d.Rand(random, d))
	return z.Bytes()
}

func (bucket *Bucket) Print(own *NodeInfo) {
	for _, ni := range bucket.Nodes {
		fmt.Printf("\t%s\n", ni)
	}
}

type Routing struct {
	ownNode *Node
	table   []*Bucket
	log     *log.Logger
}

func NewRouting(ownNode *Node) *Routing {
	routing := new(Routing)
	routing.ownNode = ownNode

	b := NewBucket(binew(0), bilsh(1, 160)) // [0, 1 << 160)
	routing.table = make([]*Bucket, 1)
	routing.table[0] = b //There is only one bucket at first

	data, err := GetPersist().LoadNodeInfo(ownNode.ID())
	if err == nil && len(data) > 0 {
		err = routing.LoadRouting(bytes.NewBuffer(data))
		if err != nil {
			panic(err)
		}
	}
	return routing
}

func (routing *Routing) Len() int {
	var length int
	for _, v := range routing.table {
		length += v.Len()
	}
	return length
}

func (routing *Routing) LoadRouting(reader io.Reader) error {
	buf := bufio.NewReader(reader)
	var data []byte = make([]byte, 24)
	_, err := buf.Read(data)
	if err != nil {
		return err
	}

	var length uint32 = 0
	err = binary.Read(bytes.NewBuffer(data[20:24]), binary.LittleEndian, &length)
	if err != nil {
		return err
	}

	var stream []byte = make([]byte, length)
	_, err = buf.Read(stream)
	if err != nil {
		return err
	}
	nodes := ParseBytesStream(stream)
	for _, v := range nodes {
		routing.InsertNode(v)
	}
	return nil
}

func (routing *Routing) Save() {
	buf := bytes.NewBuffer(nil)
	for _, v := range routing.table {
		for _, ni := range v.Nodes {
			convertNodeInfo(buf, ni)
		}
	}
	bufHeader := bytes.NewBuffer(nil)
	bufHeader.Write(routing.ownNode.ID())
	binary.Write(bufHeader, binary.LittleEndian, uint32(buf.Len()))
	bufHeader.Write(buf.Bytes())
	GetPersist().UpdateNodeInfo(routing.ownNode.ID(), bufHeader.Bytes())
}

func (routing *Routing) Print() {
	fmt.Printf("%s\n", routing.ownNode.Info.String())
	fmt.Printf("%v\n", []byte(routing.ownNode.ID()))
	for idx, bucket := range routing.table {
		fmt.Printf("#%d [%s]\n", idx, bucket.LastUpdate)
		fmt.Printf("\tMin %v\n", bucket.Min.Bytes())
		fmt.Printf("\tMax %v\n", bucket.Max.Bytes())
		bucket.Print(&routing.ownNode.Info)
	}
}

func (routing *Routing) FindNode(other Identifier, num int) []*NodeInfo {
	var result []*NodeInfo

	p := routing.bucketIndex(other)
	n := p + 1

	for len(result) < num {
		if p < 0 && n >= len(routing.table) {
			break
		}
		if p >= 0 {
			b := routing.table[p]
			b.Copy(&result, num)
		}
		if n < len(routing.table) {
			b := routing.table[n]
			b.Copy(&result, num)
		}
		p--
		n++
	}
	if len(result) > 0 {
		routing.ownNode.Log.Printf("Find %d nodes from routing", len(result))
	}
	return result
}

func (routing *Routing) InsertNode(other *NodeInfo) {
	if routing.isMe(other) {
		return
	}

	bucket, idx := routing.findBucket(other.ID)
	if bucket.UpdateIfExists(other) {
		return
	}
	if bucket.Len() < K {
		bucket.Add(other)
		return
	}
	//bucket is full
	if idx == len(routing.table)-1 {
		routing.splitBucket(bucket)
		routing.InsertNode(other)
	}
}

func (routing *Routing) isMe(other *NodeInfo) bool {
	return (routing.ownNode.Info.ID.CompareTo(other.ID) == 0)
}

func (routing *Routing) splitBucket(bucket *Bucket) {
	var last *Bucket
	fmt.Println("==========================")
	fmt.Println(bucket.Min.Bytes(), bucket.Max.Bytes())
	mid := bimid(bucket.Max, bucket.Min)
	fmt.Println(mid.Bytes())
	fmt.Println(id2bi(routing.ownNode.ID()).Bytes())
	if id2bi(routing.ownNode.ID()).Cmp(mid) >= 0 {
		fmt.Println("right")
		last = NewBucket(mid, bucket.Max)
		bucket.Max = mid
	} else {
		fmt.Println("left")
		last = NewBucket(bucket.Min, mid)
		bucket.Min = mid
	}
	fmt.Println(bucket.Min.Bytes(), bucket.Max.Bytes())
	fmt.Println(last.Min.Bytes(), last.Max.Bytes())
	routing.table = append(routing.table, last)

	var newlst []*NodeInfo
	for _, ni := range bucket.Nodes {
		idbi := id2bi(ni.ID)
		if idbi.Cmp(last.Min) >= 0 && idbi.Cmp(last.Max) < 0 {
			last.Nodes = append(last.Nodes, ni)
		} else {
			newlst = append(newlst, ni)
		}
	}
	bucket.Nodes = newlst
}

func (routing *Routing) findBucket(dst Identifier) (*Bucket, int) {
	idx := routing.bucketIndex(dst)
	length := len(routing.table)
	if length > idx {
		return routing.table[idx], idx
	}
	return routing.table[length-1], length - 1
}

func (routing *Routing) bucketIndex(dst Identifier) int {
	return BucketIndex(routing.ownNode.ID(), dst)
}
