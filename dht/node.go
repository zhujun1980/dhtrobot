package dht

import (
	"fmt"
	"io"
	"log"
	"time"
)

type Node struct {
	Info    NodeInfo
	Routing *Routing
	krpc    *KRPC
	nw      *Network
	Log     *log.Logger
	NewMsg  chan *KRPCMessage
	master  chan string
}

func NewNode(id Identifier, logger io.Writer, master chan string) *Node {
	n := new(Node)
	n.Info.ID = id
	n.Log = log.New(logger, "N ", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
	n.NewMsg = make(chan *KRPCMessage)
	n.Routing = NewRouting(n)
	n.krpc = NewKRPC(n)
	n.nw = NewNetwork(n)
	n.master = master
	return n
}

func (node *Node) ID() Identifier {
	return node.Info.ID
}

func (node *Node) Run() {
	node.Log.Printf("Current Node %s", &(node.Info))

	go func() { node.nw.StartBroker() }()
	go func() { node.nw.NetListening() }()
	go func() { node.KRPCListener() }()
	go func() { node.NodeFinder() }()

	for{
		select {
		case msg := <- node.master:
			fmt.Println(msg)
		}
	}
}

func (node *Node) KRPCListener() {
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
				closestNodes := node.Routing.FindNode(Identifier(target), K)
				nodes := ConvertByteStream(closestNodes)
				data, _ := node.krpc.EncodingNodeResult(m.T, nodes)
				node.nw.Send([]byte(data), m.Addr)
			}
			break
		case "get_peers":
			node.Log.Printf("Recv GET_PEERS %s", m.String())
			if infohash, ok := q.A["infohash"].(string); ok {
				node.Log.Printf("%s", infohash)
				//search infohash
				//not found, search nearby nodes
				//return token
			}
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
		queryNode.LastSeen = time.Now()
		node.Routing.InsertNode(queryNode)
	}
}
