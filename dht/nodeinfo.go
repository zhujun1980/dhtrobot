package dht

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"net"
	"time"
)

type Identifier []byte

func GenerateID() Identifier {
	hash := sha1.New()
	io.WriteString(hash, time.Now().String())
	return hash.Sum(nil)
}

func (id Identifier) String() string {
	return bytes.NewBuffer(id).String()
}

func (id Identifier) HexString() string {
	return fmt.Sprintf("%x", id)
}

func (id Identifier) CompareTo(other Identifier) int {
	s1 := id.String()
	s2 := other.String()
	if s1 > s2 {
		return 1
	} else if s1 == s2 {
		return 0
	} else {
		return -1
	}
}

func Distance(src, dst Identifier) []byte {
	d := make([]byte, 20)
	for i := 0; i < len(src); i++ {
		d[i] = src[i] ^ dst[i]
	}
	return d
}

//return [0, 160)
func BucketIndex(src, dst Identifier) int {
	i := 0
	for ; i < len(src); i++ {
		if src[i] != dst[i] {
			break
		}
	}
	if i == 20 {
		return 0
	}
	xor := src[i] ^ dst[i]
	j := 0
	for ; xor != 1; xor >>= 1 {
		j++
	}
	return 8*(20-(i+1)) + j
}

type NodeInfo struct {
	IP     net.IP
	Port   int
	ID     Identifier
	Status int8
}

func (ni *NodeInfo) String() string {
	s := fmt.Sprintf("[%s [%s]:%d %d]", ni.ID.HexString(), ni.IP, ni.Port, ni.Status)
	return s
}

type NodeInfos struct {
	OwnNode *Node
	NIS     []*NodeInfo
}

func (nis *NodeInfos) Len() int {
	return len(nis.NIS)
}

func (nis *NodeInfos) Less(i, j int) bool {
	ni := nis.NIS[i]
	nj := nis.NIS[j]
	d1 := fmt.Sprintf("%x", Distance(ni.ID, nis.OwnNode.Info.ID))
	d2 := fmt.Sprintf("%x", Distance(nj.ID, nis.OwnNode.Info.ID))
	return d1 < d2
}

func (nis *NodeInfos) Swap(i, j int) {
	nis.NIS[i], nis.NIS[j] = nis.NIS[j], nis.NIS[i]
}

func (nis *NodeInfos) Print() {
	for _, v := range nis.NIS {
		d := fmt.Sprintf("%x", Distance(v.ID, nis.OwnNode.Info.ID))
		fmt.Println(v, d)
	}
}

func NodesInfosToString(nodes []*NodeInfo) string {
	buf := bytes.NewBufferString("{")
	for _, v := range nodes {
		buf.WriteString(v.String())
		buf.WriteString(",")
	}
	buf.WriteString("}")
	return buf.String()
}
