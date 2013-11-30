package dht

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"sort"
	"time"
)

type Node struct {
	Info    NodeInfo
	Routing *Routing
	krpc    *KRPC
	nw      *Network
	Log     *log.Logger
	Status  int
}

func NewNode(logger io.Writer) *Node {
	n := new(Node)
	n.Info.ID = GenerateID()
	n.Routing = NewRouting(n)
	n.krpc = NewKRPC(n)
	n.nw = NewNetwork(n)
	n.Status = NEW
	n.Log = log.New(logger, "N ", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
	return n
}

func LoadNode(reader io.Reader, logger io.Writer) (*Node, error) {
	n := NewNode(logger)

	buf := bufio.NewReader(reader)
	var data []byte = make([]byte, 24)
	_, err := buf.Read(data)
	if err != nil {
		return n, err
	}

	n.Info.ID = Identifier(data[0:20])
	var length uint32 = 0
	err = binary.Read(bytes.NewBuffer(data[20:24]), binary.LittleEndian, &length)
	if err != nil {
		return n, err
	}

	var stream []byte = make([]byte, length)
	_, err = buf.Read(stream)
	if err != nil {
		return n, err
	}
	nodes := ParseBytesStream(stream)
	for _, v := range nodes {
		n.Routing.InsertNode(v)
	}
	return n, nil
}

func (node *Node) Save() {
	buf := bytes.NewBuffer(nil)
	for _, v := range node.Routing.table {
		l := v.Nodes
		for e := l.Front(); e != nil; e = e.Next() {
			ni := e.Value.(*NodeInfo)
			buf.Write(ni.ID)
			buf.Write(ni.IP)
			buf.WriteByte(byte((ni.Port & 0xFF00) >> 8))
			buf.WriteByte(byte(ni.Port & 0xFF))
		}
	}
	bufHeader := bytes.NewBuffer(nil)
	bufHeader.Write(node.Routing.ownNode.Info.ID)
	binary.Write(bufHeader, binary.LittleEndian, uint32(buf.Len()))
	bufHeader.Write(buf.Bytes())
	p := GetPersist()
	p.UpdateNodeInfo(node.Info.ID, bufHeader.Bytes())
}

func (node *Node) Run() {
	node.Log.Printf("Current Node %s", &(node.Info))
	for {
		switch node.Status {
		case NEW:
			ch := node.SearchNodes(&node.Info)
			<-ch
			break
		default:
			break
		}
	}
}

type VisitedMap map[string]bool
type Searching struct {
	ownNode *Node
	target  NodeInfo
	visited VisitedMap
	results NodeInfos
	iterNum int
	retNum  int
	d       string
	out     chan int
	in      chan int
}

func (sr *Searching) Closest() []*NodeInfo {
	var cl []*NodeInfo = nil

	for _, v := range sr.results.NIS {
		if _, ok := sr.visited[v.ID.HexString()]; !ok {
			cl = append(cl, v)
			if len(cl) == sr.retNum {
				break
			}
		}
	}
	if len(cl) > 0 {
		sr.ownNode.Log.Printf("Find nodes from results, %s", NodesInfosToString(cl))
	}
	return cl
}

func (sr *Searching) InsertNewNode(node *NodeInfo) {
	for _, v := range sr.results.NIS {
		if v.ID.CompareTo(node.ID) == 0 {
			return
		}
	}
	sr.results.NIS = append(sr.results.NIS, node)
	sort.Sort(&sr.results)
}

func (sr *Searching) IsCloseEnough() bool {
	if sr.results.NIS == nil {
		return false
	}
	cl := sr.results.NIS[0]
	newd := fmt.Sprintf("%x", Distance(sr.ownNode.Info.ID, cl.ID))
	b := false
	if sr.d != "" {
		b = (newd >= sr.d) && (sr.iterNum > 10)
		sr.ownNode.Log.Printf("Is close enough? %t, %s, %s", b, newd, sr.d)
	}
	sr.d = newd
	return b
}

func (node *Node) SearchNodes(target *NodeInfo) chan int {
	node.Log.Println("Start new searching")
	sr := new(Searching)
	go func(sr *Searching) {
		sr.ownNode = node
		sr.target = *target
		sr.visited = make(VisitedMap)
		sr.results = NodeInfos{node, nil}
		sr.iterNum = 0
		sr.d = ""
		sr.retNum = ALPHA
		sr.out = make(chan int)
		sr.in = make(chan int)
		node.search(sr)
		node.Save()
	}(sr)
	return sr.out
}

func (node *Node) BootstrapNode() []*NodeInfo {
	raddr, err := net.ResolveUDPAddr("udp", TRANSMISSIONBT)
	if err != nil {
		panic(err)
	}
	closestNodes := make([]*NodeInfo, 1)
	closestNodes[0] = &NodeInfo{raddr.IP, raddr.Port, nil, GOOD}
	node.Log.Printf("Bootstrap from %s[%s]", TRANSMISSIONBT, raddr)
	return closestNodes
}

func (node *Node) search(sr *Searching) {
	node.Log.Printf("===============iter #%d=================", sr.iterNum)
	closestNodes := sr.Closest()
	if len(closestNodes) == 0 {
		closestNodes = node.Routing.FindNode(&sr.target, sr.retNum)
		if len(closestNodes) == 0 {
			closestNodes = node.BootstrapNode()
		}
	}

	for _, v := range closestNodes {
		go func(v *NodeInfo) {
			node.sendFindNode(sr, v)
		}(v)
	}

	data := make([]byte, 500)
	for i := 0; i < len(closestNodes); i++ {
		node.nw.Conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		nread, raddr, err := node.nw.Conn.ReadFromUDP(data)
		if err != nil {
			node.Log.Printf("Read error, iter %d, %s", i, err)
		} else {
			node.Log.Printf("Read success, iter %d, %d bytes, %s", i, nread, raddr)
			rets := node.krpc.DecodeFindNodes(string(data))
			node.Log.Printf("%d nodes received", len(rets))
			for _, v1 := range rets {
				node.Log.Printf("Find new node %s", v1)
				if v1.ID.HexString() == node.Info.ID.HexString() {
					continue
				}
				node.Routing.InsertNode(v1)
				sr.InsertNewNode(v1)
			}
		}
	}

	sr.iterNum++
	if sr.IsCloseEnough() {
		node.Log.Printf("Finish searching, i=%d", sr.iterNum)
		return
	}
	node.search(sr)
}

func (node *Node) sendFindNode(sr *Searching, sn *NodeInfo) uint32 {
	raddr := &net.UDPAddr{sn.IP, sn.Port, ""}
	tid, data, err := node.krpc.FindNode(&sr.target)
	if err != nil {
		node.Log.Panic(err)
		return 0
	}
	sr.visited[sn.ID.HexString()] = true
	node.Log.Printf("Find node on %d-%s", tid, sn)
	nwrite, err := node.nw.Conn.WriteToUDP([]byte(data), raddr)
	if err != nil {
		node.Log.Panicln(err)
	} else {
		node.Log.Printf("Write %d bytes success", nwrite)
	}
	return tid
}

func (node *Node) GetPeers(infohash *string) {
}

func (node *Node) AnnouncePeer(infohash *string) {
}
