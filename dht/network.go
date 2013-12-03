package dht

import (
	"fmt"
	"net"
	"time"
)

type Network struct {
	ownNode *Node
	Conn    *net.UDPConn
	broker  *Broker
}

func NewNetwork(ownNode *Node) *Network {
	nw := new(Network)
	nw.broker = NewBroker(ownNode)
	nw.ownNode = ownNode
	nw.Init()
	nw.StartListening()
	return nw
}

func (nw *Network) Init() {
	var err error
	nw.Conn, err = net.ListenUDP("udp", nil)
	if err != nil {
		panic(err)
	}
	laddr := nw.Conn.LocalAddr().(*net.UDPAddr)
	nw.ownNode.Info.Port = laddr.Port
	nw.ownNode.Info.IP = laddr.IP
	nw.ownNode.Log.Printf("Start listening: %s", laddr)

	nw.broker.Run()
}

func (nw *Network) ReBind() {
	nw.Conn.Close()
	nw.Init()
}

func (nw *Network) StartListening() {
	go func() {
		data := make([]byte, MAXSIZE)
		for {
			nw.Conn.SetReadDeadline(time.Now().Add(10 * time.Second))
			nread, raddr, err := nw.Conn.ReadFromUDP(data)
			if err != nil {
				nw.ownNode.Log.Printf("Read error, %s", err)
				continue
			}
			nw.ownNode.Log.Printf("Read success, %d bytes, from %s", nread, raddr)
			msg, err := nw.ownNode.krpc.Decode(string(data), raddr)
			if err != nil {
				nw.ownNode.Log.Printf("Decoding error, %s", err)
			} else {
				nw.broker.PublishNewMessage(msg)
			}
		}
	}()
}

func (nw *Network) Send(m []byte, raddr *net.UDPAddr) error {
	nwrite, err := nw.Conn.WriteToUDP(m, raddr)
	if err != nil {
		nw.ownNode.Log.Printf("Send error %s", err)
	} else {
		nw.ownNode.Log.Printf("Send %d bytes success", nwrite)
	}
	return err
}

type Request struct {
	Tid      string
	Node     *Node
	SN       *NodeInfo
	Response *KRPCMessage
	ch       chan string
}

func NewRequest(tid uint32, node *Node, searchNode *NodeInfo) *Request {
	r := new(Request)
	r.ch = make(chan string, 1) //Must be buffered chan
	r.Node = node
	r.SN = searchNode
	r.Response = nil
	r.Tid = fmt.Sprintf("%x", tid)
	return r
}

type Broker struct {
	ownNode *Node
	ch      chan *Request
	chl     chan *KRPCMessage
	reqs    map[string]*Request
}

func NewBroker(ownNode *Node) *Broker {
	b := new(Broker)
	b.ownNode = ownNode
	b.reqs = make(map[string]*Request)
	b.ch = make(chan *Request)
	b.chl = make(chan *KRPCMessage)
	return b
}

func (b *Broker) Run() {
	go func() {
		for {
			b.ownNode.Log.Printf("Broker listening")
			select {
			case r := <-b.ch:
				b.ownNode.Log.Printf("Broker recv request #%s", r.Tid)
				b.reqs[r.Tid] = r
			case m := <-b.chl:
				if req, ok := b.reqs[m.T]; ok {
					b.ownNode.Log.Printf("Broker recv response #%s", m.T)
					req.Response = m
					req.ch <- m.T
					b.ownNode.Log.Printf("Broker dispatchs over")
					delete(b.reqs, m.T)
				} else {
					b.ownNode.Log.Printf("Broker recv query #%s", m.T)
					b.ownNode.NewMsg <- m
				}
			case <-time.After(5 * time.Second):
				//gc
				b.ownNode.Log.Printf("Broker timeout")
			}
		}
	}()
}

func (b *Broker) AddRequest(r *Request) {
	b.ownNode.Log.Printf("Add request #%s start", r.Tid)
	b.ch <- r
	b.ownNode.Log.Printf("Add request #%s end", r.Tid)
}

func (b *Broker) WaitResponse(rs []*Request, t time.Duration) chan string {
	ch := make(chan string, len(rs))

	for _, r := range rs {
		go func(r *Request) {
			r.Node.Log.Printf("Wait response #%s", r.Tid)
			select {
			case s := <-r.ch:
				ch <- s
				r.Node.Log.Printf("Response #%s return", r.Tid)
				return
			case <-time.After(t):
				r.Node.Log.Printf("Wait timeout #%s", r.Tid)
				ch <- ""
				return
			}
		}(r)
	}
	return ch
}

func (b *Broker) PublishNewMessage(m *KRPCMessage) {
	b.ownNode.Log.Printf("Message publish #%s start", m.T)
	b.chl <- m
	b.ownNode.Log.Printf("Message publish #%s end", m.T)
}
