package dht

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

type Node struct {
	Info    NodeInfo
	Routing *Routing
	krpc    *KRPC
	nw      *Network
	Log     *log.Logger
	MLog    *log.Logger
	NewMsg  chan *KRPCMessage
	master  chan string
	tokens  map[string]*TokenVal
	secret  string
}

func NewNode(id Identifier, logger io.Writer, mlogger io.Writer, master chan string) *Node {
	n := new(Node)
	n.Info.ID = id
	n.Log = log.New(logger, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
	n.MLog = log.New(mlogger, id.HexString()+" ", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
	n.NewMsg = make(chan *KRPCMessage)
	n.Routing = NewRouting(n)
	n.krpc = NewKRPC(n)
	n.nw = NewNetwork(n)
	n.master = master
	n.tokens = make(map[string]*TokenVal)
	n.Info.LastSeen = time.Now()
	return n
}

func (node *Node) ID() Identifier {
	return node.Info.ID
}

func (node *Node) Cli() {
	node.Log.Printf("Current Node %s", &(node.Info))

	go func() { node.nw.StartBroker() }()
	go func() { node.nw.NetListening() }()
	go func() { node.KRPCListener() }()
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
			node.MLog.Printf("Recv PING #%s, %s", m.T, queryNode)
			data, _ := node.krpc.EncodeingPong(m.T)
			node.nw.Send([]byte(data), m.Addr)
			node.Routing.Print()
			break
		case "find_node":
			node.MLog.Printf("Recv FIND_NODE #%s %s", m.T, m.String())
			if target, ok := q.A["target"].(string); ok {
				closestNodes := node.Routing.FindNode(Identifier(target), K)
				nodes := ConvertByteStream(closestNodes)
				data, _ := node.krpc.EncodingNodeResult(m.T, "", nodes)
				node.nw.Send([]byte(data), m.Addr)
			}
			break
		case "get_peers":
			node.MLog.Printf("Recv GET_PEERS #%s %s", m.T, m.String())
			if infohash, ok := q.A["info_hash"].(string); ok {
				ih := Identifier(infohash)
				GetPersist().AddResource(ih.HexString())
				token := node.GenToken(queryNode)
				peers, _ := GetPersist().LoadPeers(ih.HexString())
				if len(peers) > 0 {
					data, _ := node.krpc.EncodingPeerResult(m.T, token, peers)
					node.nw.Send([]byte(data), m.Addr)
				} else {
					closestNodes := node.Routing.FindNode(ih, K)
					nodes := ConvertByteStream(closestNodes)
					data, _ := node.krpc.EncodingNodeResult(m.T, token, nodes)
					node.nw.Send([]byte(data), m.Addr)
				}
			}
			break
		case "announce_peer":
			node.MLog.Printf("Recv ANNOUNCE_PEER #%s %s", m.T, m.String())
			var infohash string
			var token string
			var implied_port int64
			var port int64
			var ok bool
			var tv *TokenVal
			if infohash, ok = q.A["info_hash"].(string); !ok {
				break
			}
			if token, ok = q.A["token"].(string); !ok {
				break
			}
			if port, ok = q.A["port"].(int64); !ok {
				break
			}
			if implied_port, ok = q.A["implied_port"].(int64); !ok {
				implied_port = 0
			}
			if implied_port > 0 {
				port = int64(m.Addr.Port)
			}
			ih := Identifier(infohash)
			tv, ok = node.tokens[token]
			if ok && tv.node.IP.Equal(queryNode.IP) {
				buf := bytes.NewBufferString("")
				convertIPPort(buf, queryNode.IP, int(port))
				GetPersist().AddPeer(ih.HexString(), buf.Bytes())
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

func (node *Node) Ping(raddr *net.UDPAddr) error {
	tid, data, err := node.krpc.EncodeingPing()
	if err != nil {
		node.Log.Printf("%s", err)
		return err
	}
	r := NewRequest(tid, node, nil)
	node.nw.broker.AddRequest(r)
	node.nw.Send([]byte(data), raddr)
	var reqs []*Request
	reqs = append(reqs, r)
	ch := FanInRequests(reqs, time.Second*3)
	req := <-ch
	if req != nil {
		if res, ok := req.Response.Addion.(*Response); ok {
			if nodestr, ok := res.R["id"].(string); ok {
				node.Log.Printf("PING ok #%s %s", req.Tid, Identifier(nodestr).HexString())
			}
		}
	}
	return nil
}

func (node *Node) FindNode(raddr *net.UDPAddr, target Identifier) {
	tid, data, err := node.krpc.EncodingFindNode(target)
	if err != nil {
		node.Log.Fatalln(err)
		return
	}
	r := NewRequest(tid, node, nil)
	node.nw.broker.AddRequest(r)
	err = node.nw.Send([]byte(data), raddr)
	var reqs []*Request
	reqs = append(reqs, r)
	ch := FanInRequests(reqs, time.Second*3)
	req := <-ch
	if req != nil {
		if res, ok := req.Response.Addion.(*Response); ok {
			if nodestr, ok := res.R["nodes"].(string); ok {
				nodes := ParseBytesStream([]byte(nodestr))
				node.Log.Printf("%d nodes received", len(nodes))
				node.Log.Printf("%s", NodesInfosToString(nodes))
			}
		}
	}
	return
}

func (node *Node) GetPeersAndAnnounce(raddr *net.UDPAddr, infohash Identifier) {
	tid, data, err := node.krpc.EncodingGetPeers(infohash)
	if err != nil {
		node.Log.Fatalln(err)
		return
	}
	r := NewRequest(tid, node, nil)
	node.nw.broker.AddRequest(r)
	err = node.nw.Send([]byte(data), raddr)
	var reqs []*Request
	reqs = append(reqs, r)
	ch := FanInRequests(reqs, time.Second*3)
	req := <-ch
	if req != nil {
		if res, ok := req.Response.Addion.(*Response); ok {
			if token, ok := res.R["token"].(string); ok {
				node.Log.Printf("%s", token)
				if nodestr, ok := res.R["nodes"].(string); ok {
					nodes := ParseBytesStream([]byte(nodestr))
					node.Log.Printf("%d nodes received", len(nodes))
					node.Log.Printf("%s", NodesInfosToString(nodes))
				} else if peers, ok := res.R["values"].([]interface{}); ok {
					node.Log.Printf("%d peers received", len(peers))
					for _, peer := range peers {
						p := []byte(peer.(string))
						ip := p[0:4]
						pt := p[4:6]
						port := int(pt[0])<<8 + int(pt[1])
						addr := &net.UDPAddr{ip, port, ""}
						node.Log.Printf("%s", addr)
					}
				}
				tid, data, err = node.krpc.EncodingAnnouncePeer(infohash, node.Info.Port, token)
				err = node.nw.Send([]byte(data), raddr)
			}
		}
	}
	return
}
