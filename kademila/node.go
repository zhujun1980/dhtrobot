package kademila

import (
	"crypto/sha1"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strconv"
	"time"
)

type NodeID []byte

func GenerateID() NodeID {
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	hash := sha1.New()
	io.WriteString(hash, time.Now().String())
	io.WriteString(hash, string(random.Int()))
	return hash.Sum(nil)
}

func (id NodeID) String() string {
	return string(id)
}

func (id NodeID) HexString() string {
	return fmt.Sprintf("%x", id)
}

func HexToID(hex string) NodeID {
	if len(hex) != 40 {
		return nil
	}
	id := make([]byte, 20)
	j := 0
	for i := 0; i < len(hex); i += 2 {
		n1, _ := strconv.ParseInt(hex[i:i+1], 16, 8)
		n2, _ := strconv.ParseInt(hex[i+1:i+2], 16, 8)
		id[j] = byte((n1 << 4) + n2)
		j++
	}
	return id
}

var lookup = []int{4, 3, 2, 2, 1, 1, 1, 1,
	0, 0, 0, 0, 0, 0, 0, 0}

func BitsInByte(b byte) int {
	f := lookup[b>>4]
	if f == 4 {
		return f + lookup[b&0xF]
	}
	return f
}

func (id NodeID) Distance(dst NodeID) int {
	return Distance(id, dst)
}

func Distance(src, dst NodeID) int {
	var d int
	for i := 0; i < len(src); i++ {
		b := src[i] ^ dst[i]
		if b == 0 {
			d += 8
		} else {
			d += BitsInByte(b)
			break
		}
	}
	return MaxBitsLength - d
}

type Node struct {
	ID     NodeID
	Addr   net.Addr
	Status uint8
}

func (node Node) IP() []byte {
	return node.Addr.(*net.UDPAddr).IP
}

func (node Node) Port() int {
	return node.Addr.(*net.UDPAddr).Port
}

func (node Node) String() string {
	if node.Addr != nil {
		return fmt.Sprintf("ID=%s, Addr=%s, Status=%d", node.ID.HexString(), node.Addr.String(), node.Status)
	}
	return fmt.Sprintf("ID=%s, Addr=, Status=%d", node.ID.HexString(), node.Status)
}

func (node *Node) Clone() *Node {
	n := new(Node)
	n.ID = make([]byte, len(node.ID))
	copy(n.ID, node.ID)
	n.Addr = node.Addr
	n.Status = node.Status
	return n
}
