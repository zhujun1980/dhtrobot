package kademila

import (
	"fmt"
	"net"
)

type Peer struct {
	Port int
	IP   net.IP
}

func (peer Peer) String() string {
	return fmt.Sprintf("Addr=%s:%d", peer.IP.String(), peer.Port)
}
