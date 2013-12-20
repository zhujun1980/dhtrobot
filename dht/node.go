package dht

import (
	"bytes"
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
	tokens  map[string]*TokenVal
	secret  string
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
	n.tokens = make(map[string]*TokenVal)
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

	for {
		select {
		case msg := <-node.master:
			fmt.Println(msg)
		case <-time.After(time.Minute * 5):
			node.secret = GenerateID().HexString()
			GetPersist().DeleteOldPeers()
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
		queryNode := new(NodeInfo)
		queryNode.IP = m.Addr.IP
		queryNode.Port = m.Addr.Port
		queryNode.Status = GOOD
		queryNode.ID = Identifier(q.A["id"].(string))
		queryNode.LastSeen = time.Now()

		switch q.Y {
		case "ping":
			node.Log.Printf("Recv PING %s", m.String())
			data, _ := node.krpc.EncodeingPong(m.T)
			node.Log.Printf("Pong result %s", data)
			node.nw.Send([]byte(data), m.Addr)
			break
		case "find_node":
			node.Log.Printf("Recv FIND_NODE %s", m.String())
			if target, ok := q.A["target"].(string); ok {
				closestNodes := node.Routing.FindNode(Identifier(target), K)
				nodes := ConvertByteStream(closestNodes)
				data, _ := node.krpc.EncodingNodeResult(m.T, "", nodes)
				node.Log.Printf("Find nodes result %s", data)
				node.nw.Send([]byte(data), m.Addr)
			}
			break
		case "get_peers":
			node.Log.Printf("Recv GET_PEERS %s", m.String())
			if infohash, ok := q.A["infohash"].(string); ok {
				node.Log.Printf("%s", infohash)
				ih := Identifier(infohash)
				GetPersist().AddResource(ih.HexString())
				token := node.GenToken(queryNode)
				peers, _ := GetPersist().LoadPeers(ih.HexString())
				if len(peers) > 0 {
					data, _ := node.krpc.EncodingPeerResult(m.T, token, peers)
					node.Log.Printf("Get peer result %s", data)
					node.nw.Send([]byte(data), m.Addr)
				} else {
					closestNodes := node.Routing.FindNode(ih, K)
					nodes := ConvertByteStream(closestNodes)
					data, _ := node.krpc.EncodingNodeResult(m.T, token, nodes)
					node.Log.Printf("Get peer result %s", data)
					node.nw.Send([]byte(data), m.Addr)
				}
			}
			break
		case "announce_peer":
			node.Log.Printf("Recv ANNOUNCE_PEER %s", m.String())
			var infohash string
			var token string
			var implied_port int
			var port int
			var ok bool
			var tv *TokenVal
			if infohash, ok = q.A["infohash"].(string); !ok {
				break
			}
			if token, ok = q.A["token"].(string); !ok {
				break
			}
			if port, ok = q.A["port"].(int); !ok {
				break
			}
			if implied_port, ok = q.A["implied_port"].(int); !ok {
				implied_port = 0
			}
			if implied_port > 0 {
				port = m.Addr.Port
			}
			node.Log.Printf("%s-%s-%d-%d", infohash, token, implied_port, port)
			tv, ok = node.tokens[token]
			if ok && tv.node.IP.Equal(queryNode.IP) {
				buf := bytes.NewBuffer(nil)
				convertIPPort(buf, queryNode.IP, port)
				GetPersist().AddPeer(infohash, buf.Bytes())
			}
			if ok {
				delete(node.tokens, token)
			}
			data, _ := node.krpc.EncodeingPong(m.T)
			node.nw.Send([]byte(data), m.Addr)
			break
		}
		node.Routing.InsertNode(queryNode)
	}
}
