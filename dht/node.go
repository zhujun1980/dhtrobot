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
	NewMsg  chan *KRPCMessage
}

func NewNode(logger io.Writer) *Node {
	n := new(Node)
	n.Log = log.New(logger, "N ", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
	n.NewMsg = make(chan *KRPCMessage)
	n.Info.ID = GenerateID()
	n.Routing = NewRouting(n)
	n.krpc = NewKRPC(n)
	n.nw = NewNetwork(n)
	n.Status = NEW
	return n
}

func LoadNode(reader io.Reader, logger io.Writer) (*Node, error) {
	n := NewNode(logger)
	n.Status = SENIOR

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
	GetPersist().UpdateNodeInfo(node.Info.ID, bufHeader.Bytes())
}

func (node *Node) Run() {
	node.Log.Printf("Current Node %s", &(node.Info))

	go func() { node.StartListener() }()
	go func() { node.StartNodeChecker() }()
}

func (node *Node) StartListener() {
	for {
		select {
		case m := <-node.NewMsg:
			if m.Y == "q" {
				node.ProcessQuery(m)
			}
		}
	}
}

func (node *Node) ProcessQuery(m *KRPCMessage) {
	if q, ok := m.Addion.(*Query); ok {
		switch q.Y {
		case "ping":
			node.Log.Printf("Recv PING %s", m.String())
			data, _ := node.krpc.EncodeingPong(m.T)
			node.nw.Send([]byte(data), m.Addr)
			break
		case "find_node":
			node.Log.Printf("Recv FIND_NODE %s", m.String())
			if target, ok := q.A["target"].(string); ok {
				targetNode := &NodeInfo{nil, 0, Identifier(target), 0}
				closestNodes := node.Routing.FindNode(targetNode, K)
				nodes := ConvertByteStream(closestNodes)
				data, _ := node.krpc.EncodingNodeResult(m.T, nodes)
				node.nw.Send([]byte(data), m.Addr)
			}
			break
		case "get_peers":
			node.Log.Printf("Recv GET_PEERS %s", m.String())
			break
		case "announce_peer":
			node.Log.Printf("Recv ANNOUNCE_PEER %s", m.String())
			break
		}

		queryNode := new(NodeInfo)
		queryNode.IP = m.Addr.IP
		queryNode.Port = m.Addr.Port
		queryNode.Status = GOOD
		queryNode.ID = Identifier(q.A["id"].(string))
		node.Routing.InsertNode(queryNode)
	}
}

func (node *Node) StartNodeChecker() {
	if node.Status == NEW {
		node.SearchNodes(&node.Info)
	} else {
		node.RefreshBucket(true)
	}
	node.Save()

	for {
		select {
		case <-time.After(10 * time.Second):
			node.RefreshBucket(false)
			node.Save()
		}
	}
}

func (node *Node) RefreshBucket(force bool) {
	i := 1
	for k, v := range node.Routing.table {
		if force || v.LastUpdate.Add(EXPIRE_DURATION).Before(time.Now()) {
			if v.Nodes.Len() == 0 {
				continue
			}
			node.Log.Printf("Bucket expired, #%d k=%d [%s][%s]",
				i, k, v.LastUpdate, time.Now())
			ni := v.Nodes.Front().Value.(*NodeInfo)
			go func(v *NodeInfo) {
				node.SearchNodes(v)
			}(ni)
			i++
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
		b = (newd >= sr.d) && (sr.iterNum > 5)
		sr.ownNode.Log.Printf("Is close enough? %t, %s, %s", b, newd, sr.d)
	}
	sr.d = newd
	return b
}

func (node *Node) SearchNodes(target *NodeInfo) {
	node.Log.Println("Start new searching")
	sr := new(Searching)
	sr.ownNode = node
	sr.target = *target
	sr.visited = make(VisitedMap)
	sr.results = NodeInfos{node, nil}
	sr.iterNum = 0
	sr.d = ""
	sr.retNum = ALPHA
	node.search(sr)
}

func (node *Node) BootstrapNode() []*NodeInfo {
	raddr, err := net.ResolveUDPAddr("udp", TRANSMISSIONBT)
	if err != nil {
		node.Log.Printf("Boot error %s", err)
		return nil
	}
	closestNodes := make([]*NodeInfo, 1)
	closestNodes[0] = &NodeInfo{raddr.IP, raddr.Port, nil, GOOD}
	node.Log.Printf("Bootstrap from %s[%s]", TRANSMISSIONBT, raddr)
	return closestNodes
}

func (node *Node) search(sr *Searching) {
	node.Log.Printf("===============iter #%d[%s]=================", sr.iterNum, sr.target.ID.HexString())
	//TODO FIND ME CHECKING
	closestNodes := sr.Closest()
	if len(closestNodes) == 0 {
		closestNodes = node.Routing.FindNode(&sr.target, sr.retNum)
		if len(closestNodes) == 0 {
			closestNodes = node.BootstrapNode()
		}
	}

	var reqs []*Request
	for _, v := range closestNodes {
		r := node.sendFindNode(sr, v)
		if r != nil {
			reqs = append(reqs, r)
		}
	}

	var ch chan string
	if len(reqs) > 0 {
		ch = node.nw.broker.WaitResponse(reqs, time.Second*10)
		for i := 0; i < len(reqs); i++ {
			tid := <-ch
			if tid != "" {
				for _, req := range reqs {
					if req.Tid != tid {
						continue
					}
					res, ok := req.Response.Addion.(*Response)
					if !ok {
						node.Log.Printf("Response is invalid")
						continue
					}
					nodestr, ok := res.R["nodes"].(string)
					if !ok {
						node.Log.Printf("Nodes isn't string")
						continue
					}
					nodes := []byte(nodestr)
					rets := ParseBytesStream(nodes)
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
		}
		for _, v := range reqs {
			if v.Response != nil {
				node.Routing.UpNode(v.SN)
			} else {
				node.Routing.DownNode(v.SN)
			}
		}
	}

	sr.iterNum++
	if sr.IsCloseEnough() {
		node.Log.Printf("Finish searching, i = %d", sr.iterNum)
		return
	}
	node.search(sr)
}

func (node *Node) sendFindNode(sr *Searching, sn *NodeInfo) *Request {
	raddr := &net.UDPAddr{sn.IP, sn.Port, ""}
	tid, data, err := node.krpc.EncodingFindNode(&sr.target)
	if err != nil {
		node.Log.Panic(err)
		return nil
	}

	r := NewRequest(tid, node, sn)
	node.nw.broker.AddRequest(r) //Add request before send it

	sr.visited[sn.ID.HexString()] = true
	node.Log.Printf("Find node on #%x-%s", tid, sn)

	err = node.nw.Send([]byte(data), raddr)
	if err != nil {
		return nil
	}
	return r
}

func (node *Node) GetPeers(infohash *string) {
}

func (node *Node) AnnouncePeer(infohash *string) {
}
