package dht

import (
	"fmt"
	"net"
	"time"
)

type Network struct {
	ownNode *Node
	Conn    *net.UDPConn
}

func NewNetwork(own *Node) *Network {
	nw := new(Network)
	nw.ownNode = own
	nw.Init()
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
	fmt.Println("Start listening:", laddr)
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
				fmt.Println(err)
				continue
			}
			fmt.Println("recv:", nread, raddr)
			nw.ownNode.krpc.Decode(string(data))
			//node.recv <- data
		}
	}()
}


