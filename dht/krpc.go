package dht

import (
	"bytes"
	"fmt"
	"github.com/zeebo/bencode"
	"math"
	"net"
	"sync/atomic"
	"time"
)

type KRPC struct {
	ownNode *Node
	tid     uint32
}

func NewKRPC(ownNode *Node) *KRPC {
	krpc := new(KRPC)
	krpc.ownNode = ownNode
	return krpc
}

type Query struct {
	Y string
	A map[string]interface{}
}

func (q *Query) String() string {
	buf := bytes.NewBufferString("[")
	buf.WriteString(q.Y)
	buf.WriteString(" ")
	id := Identifier(q.A["id"].(string))
	buf.WriteString(id.HexString())

	switch q.Y {
	case "ping":
		break
	case "find_node":
		target := Identifier(q.A["target"].(string))
		buf.WriteString(fmt.Sprintf(" %s", target.HexString()))
		break
	case "get_peers":
		info_hash := Identifier(q.A["info_hash"].(string))
		buf.WriteString(fmt.Sprintf(" %s", info_hash.HexString()))
		break
	case "announce_peer":
		implied_port := q.A["implied_port"].(int64)
		info_hash := Identifier(q.A["info_hash"].(string))
		port := q.A["port"].(int64)
		buf.WriteString(fmt.Sprintf("%d %s %d",
			implied_port, info_hash.HexString(), port))
		break
	}
	buf.WriteString("]")
	return buf.String()
}

type Response struct {
	R map[string]interface{}
}

func (r *Response) String() string {
	buf := bytes.NewBufferString("[")
	buf.WriteString("]")
	return buf.String()
}

type Err struct {
	E []interface{}
}

func (e *Err) String() string {
	buf := bytes.NewBufferString("[")
	buf.WriteString(fmt.Sprintf("%v", e))
	buf.WriteString("]")
	return buf.String()
}

type KRPCMessage struct {
	T      string
	Y      string
	Addion interface{}
	Addr   *net.UDPAddr
}

func (m *KRPCMessage) String() string {
	buf := bytes.NewBufferString("{")
	buf.WriteString(fmt.Sprintf("#%s %s ", m.T, m.Y))
	switch m.Y {
	case "q":
		q := m.Addion.(*Query)
		buf.WriteString(q.String())
		break
	case "r":
		r := m.Addion.(*Response)
		buf.WriteString(r.String())
		break
	case "e":
		e := m.Addion.(*Err)
		buf.WriteString(e.String())
		break
	}

	buf.WriteString("}")
	return buf.String()
}

func (encode *KRPC) GenTID() uint32 {
	return encode.autoID() % math.MaxUint16
}

func (encode *KRPC) autoID() uint32 {
	return atomic.AddUint32(&encode.tid, 1)
}

func convertIPPort(buf *bytes.Buffer, ip net.IP, port int) {
	buf.Write(ip.To4())
	buf.WriteByte(byte((port & 0xFF00) >> 8))
	buf.WriteByte(byte(port & 0xFF))
}

func convertNodeInfo(buf *bytes.Buffer, v *NodeInfo) {
	buf.Write(v.ID)
	convertIPPort(buf, v.IP, v.Port)
}

func ConvertByteStream(nodes []*NodeInfo) []byte {
	buf := bytes.NewBuffer(nil)
	for _, v := range nodes {
		convertNodeInfo(buf, v)
	}
	return buf.Bytes()
}

func ParseBytesStream(data []byte) []*NodeInfo {
	var nodes []*NodeInfo = nil
	for j := 0; j < len(data); j = j + 26 {
		if j+26 > len(data) {
			break
		}

		kn := data[j : j+26]
		node := new(NodeInfo)
		node.ID = Identifier(kn[0:20])
		node.IP = kn[20:24]
		port := kn[24:26]
		node.Port = int(port[0])<<8 + int(port[1])
		node.Status = GOOD
		node.LastSeen = time.Now()

		nodes = append(nodes, node)
	}
	return nodes
}

func (encode *KRPC) Decode(message string, raddr *net.UDPAddr) (*KRPCMessage, error) {
	val := make(map[string]interface{})

	if err := bencode.DecodeString(message, &val); err != nil {
		return nil, err
	} else {
		message := new(KRPCMessage)
		message.T = val["t"].(string)
		message.Y = val["y"].(string)
		message.Addr = raddr
		switch message.Y {
		case "q": //query recv from other node
			query := new(Query)
			query.Y = val["q"].(string)
			query.A = val["a"].(map[string]interface{})
			message.Addion = query
			break
		case "r": //response
			res := new(Response)
			res.R = val["r"].(map[string]interface{})
			message.Addion = res
			break
		case "e": //error
			err := new(Err)
			err.E = val["e"].([]interface{})
			message.Addion = err
			break
		default:
			fmt.Println("invalid message")
			break
		}
		return message, nil
	}
}

func (encode *KRPC) EncodingFindNode(target Identifier) (uint32, string, error) {
	tid := encode.GenTID()
	v := make(map[string]interface{})
	v["t"] = fmt.Sprintf("%d", tid)
	v["y"] = "q"
	v["q"] = "find_node"
	args := make(map[string]string)
	args["id"] = encode.ownNode.Info.ID.String()
	args["target"] = target.String()
	v["a"] = args
	s, err := bencode.EncodeString(v)
	return tid, s, err
}

func (encode *KRPC) EncodingGetPeers(infohash Identifier) (uint32, string, error) {
	tid := encode.GenTID()
	v := make(map[string]interface{})
	v["t"] = fmt.Sprintf("%d", tid)
	v["y"] = "q"
	v["q"] = "get_peers"
	args := make(map[string]string)
	args["id"] = encode.ownNode.Info.ID.String()
	args["info_hash"] = infohash.String()
	v["a"] = args
	s, err := bencode.EncodeString(v)
	return tid, s, err
}

func (encode *KRPC) EncodingAnnouncePeer(infohash Identifier, port int, token string) (uint32, string, error) {
	tid := encode.GenTID()
	v := make(map[string]interface{})
	v["t"] = fmt.Sprintf("%d", tid)
	v["y"] = "q"
	v["q"] = "announce_peer"
	args := make(map[string]interface{})
	args["id"] = encode.ownNode.Info.ID.String()
	args["token"] = token
	args["port"] = port
	args["implied_port"] = 0
	args["info_hash"] = infohash.String()
	v["a"] = args
	s, err := bencode.EncodeString(v)
	return tid, s, err
}

func (encode *KRPC) EncodeingPing() (uint32, string, error) {
	tid := encode.GenTID()
	v := make(map[string]interface{})
	v["t"] = fmt.Sprintf("%d", tid)
	v["y"] = "q"
	v["q"] = "ping"
	args := make(map[string]string)
	args["id"] = encode.ownNode.Info.ID.String()
	v["a"] = args
	s, err := bencode.EncodeString(v)
	return tid, s, err
}

func (encode *KRPC) EncodeingPong(tid string) (string, error) {
	v := make(map[string]interface{})
	v["t"] = fmt.Sprintf("%s", tid)
	v["y"] = "r"
	args := make(map[string]string)
	args["id"] = encode.ownNode.Info.ID.String()
	v["r"] = args
	s, err := bencode.EncodeString(v)
	return s, err
}

func (encode *KRPC) EncodingNodeResult(tid string, token string, nodes []byte) (string, error) {
	v := make(map[string]interface{})
	v["t"] = tid
	v["y"] = "r"
	args := make(map[string]string)
	args["id"] = encode.ownNode.Info.ID.String()
	if token != "" {
		args["token"] = token
	}
	args["nodes"] = bytes.NewBuffer(nodes).String()
	v["r"] = args
	s, err := bencode.EncodeString(v)
	return s, err
}

func (encode *KRPC) EncodingPeerResult(tid string, token string, peers []string) (string, error) {
	v := make(map[string]interface{})
	v["t"] = tid
	v["y"] = "r"
	args := make(map[string]interface{})
	args["id"] = encode.ownNode.Info.ID.String()
	args["token"] = token
	args["values"] = peers
	v["r"] = args
	s, err := bencode.EncodeString(v)
	return s, err
}
