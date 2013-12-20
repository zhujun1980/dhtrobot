package dht

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
	"math/big"
	"math/rand"
)

type Identifier []byte

func GenerateID() Identifier {
	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	hash := sha1.New()
	io.WriteString(hash, time.Now().String())
	io.WriteString(hash, string(random.Int()))
	return hash.Sum(nil)
}

func HexToID(hex string) Identifier {
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

func (id Identifier) String() string {
	return bytes.NewBuffer(id).String()
}

func (id Identifier) HexString() string {
	return fmt.Sprintf("%x", id)
}

func (id Identifier) CompareTo(other Identifier) int {
	s1 := id.HexString()
	s2 := other.HexString()
	if s1 > s2 {
		return 1
	} else if s1 == s2 {
		return 0
	} else {
		return -1
	}
}

func RandID(src Identifier, diff int) Identifier {
	ret := big.NewInt(0).SetBytes(GenerateID())
	srcNum := big.NewInt(0).SetBytes(src)
	i := srcNum.Bit(diff) ^ 1
	ret.SetBit(ret, diff, i)
	for j := diff + 1; j < 160; j++ {
		ret.SetBit(ret, j, srcNum.Bit(j))
	}

	return ret.Bytes()
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
	IP       net.IP
	Port     int
	ID       Identifier
	Status   int8
	LastSeen time.Time
}

func (ni *NodeInfo) String() string {
	s := fmt.Sprintf("[%s [%s]:%d %d %s]",
		ni.ID.HexString(), ni.IP, ni.Port, ni.Status, ni.LastSeen)
	return s
}

func (ni *NodeInfo) Touch() {
	ni.LastSeen = time.Now()
}

type NodeInfos struct {
	Target Identifier
	NIS    []*NodeInfo
}

func (nis *NodeInfos) Len() int {
	return len(nis.NIS)
}

func (nis *NodeInfos) Less(i, j int) bool {
	ni := nis.NIS[i]
	nj := nis.NIS[j]
	d1 := fmt.Sprintf("%x", Distance(ni.ID, nis.Target))
	d2 := fmt.Sprintf("%x", Distance(nj.ID, nis.Target))
	return d1 < d2
}

func (nis *NodeInfos) Swap(i, j int) {
	nis.NIS[i], nis.NIS[j] = nis.NIS[j], nis.NIS[i]
}

func (nis *NodeInfos) Print() {
	for _, v := range nis.NIS {
		d := fmt.Sprintf("%x", Distance(v.ID, nis.Target))
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
