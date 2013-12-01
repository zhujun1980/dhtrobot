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

func NewNetwork(own *Node) *Network {
	nw := new(Network)
	nw.broker = NewBroker()
	nw.ownNode = own
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
	//nw.ownNode.Log.Printf("Start listening: %s", laddr)

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
			msg, err := nw.ownNode.krpc.Decode(string(data))
			if err != nil {
				nw.ownNode.Log.Printf("Decoding error, %s", err)
			} else {
				nw.broker.PublishNewMessage(msg)
			}
		}
	}()
}

type Request struct {
	Tid      string
	Node     *Node
	Response *KRPCMessage
	ch       chan string
}

func NewRequest(tid uint32, node *Node) *Request {
	r := new(Request)
	r.ch = make(chan string, 1)
	r.Node = node
	r.Response = nil
	r.Tid = fmt.Sprintf("%x", tid)
	return r
}

type Broker struct {
	ch   chan *Request
	chl  chan *KRPCMessage
	reqs map[string]*Request
}

func NewBroker() *Broker {
	b := new(Broker)
	b.reqs = make(map[string]*Request)
	b.ch = make(chan *Request)
	b.chl = make(chan *KRPCMessage)
	return b
}

func (b *Broker) Run() {
	go func() {
		for {
			fmt.Println("broker start")
			select {
			case r := <-b.ch:
				fmt.Println("broker r0")
				b.reqs[r.Tid] = r
				fmt.Println("broker r1")
			case m := <-b.chl:
				fmt.Println("broker r2", m.T)
				fmt.Println(m.String())
				if req, ok := b.reqs[m.T]; ok {
					fmt.Println(1)
					req.Response = m
					fmt.Println(2)
					req.ch <- m.T
					fmt.Println("broker r3")
					delete(b.reqs, m.T)
				} else {
					fmt.Println(3)
					fmt.Println("broker r4", m.String())
				}
			case <-time.After(5 * time.Second):
				//gc
				fmt.Println("broker timeout")
			}
		}
	}()
}

func (b *Broker) AddRequest(r *Request) {
	fmt.Println("add Response start")
	b.ch <- r
	fmt.Println("add Response end")
}

func (b *Broker) WaitResponse(rs []*Request, t time.Duration) chan string {
	ch := make(chan string, len(rs))

	for _, r := range rs {
		go func(r *Request) {
			r.Node.Log.Printf("Wait response #%s", r.Tid)
			select {
			case s := <-r.ch:
				ch <- s
				r.Node.Log.Printf("Response #%s", r.Tid)
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
	fmt.Println("publish start")
	b.chl <- m
	fmt.Println("publish end")
}
