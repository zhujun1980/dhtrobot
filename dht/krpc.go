package dht

import (
	"bytes"
	"fmt"
	"github.com/zeebo/bencode"
	"math"
	"net"
	"sync/atomic"
)

type Action func(ctx interface{}, message *KRPCMessage)
type CallBack struct {
	name   string
	ctx    interface{}
	action Action
}
type Listener map[string]*CallBack

func (krpc *KRPC) Register(name string, action Action, ctx interface{}) {
	cb := new(CallBack)
	cb.name = name
	cb.ctx = ctx
	cb.action = action
	krpc.listener[name] = cb
}

func (krpc *KRPC) UnRegister(name string) {
	delete(krpc.listener, name)
}

type KRPC struct {
	ownNode  *Node
	tid      uint32
	listener Listener
}

func NewKRPC(ownNode *Node) *KRPC {
	krpc := new(KRPC)
	krpc.ownNode = ownNode
	krpc.listener = make(Listener)
	return krpc
}

type Query struct {
	Y string
	A map[string]interface{}
}

func (q *Query) String() string {
	buf := bytes.NewBufferString("[")
	buf.WriteString(q.Y)
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
		implied_port := q.A["implied_port"].(int)
		info_hash := Identifier(q.A["info_hash"].(string))
		port := q.A["port"].(int)
		token := q.A["token"].(string)
		buf.WriteString(fmt.Sprintf("%d %s %d %s",
			implied_port, info_hash.HexString(), port, token))
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

func ConvertByteStream(nodes []*NodeInfo) []byte {
	buf := bytes.NewBufferString("")
	for _, v := range nodes {
		buf.Write(v.ID)
		buf.Write(v.IP)
		buf.WriteByte(byte((v.Port & 0xFF00) >> 8))
		buf.WriteByte(byte(v.Port & 0xFF))
	}
	return buf.Bytes()
}

func ParseBytesStream(data []byte) []*NodeInfo {
	var nodes []*NodeInfo = nil
	for j := 0; j < len(data); j = j + 26 {
		kn := data[j : j+26]
		id := kn[0:20]
		ip := kn[20:24]
		port := kn[24:26]
		var p int = int(port[0])<<8 + int(port[1])
		nodes = append(nodes, &NodeInfo{ip, p, Identifier(id), GOOD})
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
		if cb, ok := encode.listener[message.T]; ok {
			cb.action(cb.ctx, message)
		}
		return message, nil
	}
}

func (encode *KRPC) EncodingFindNode(target *NodeInfo) (uint32, string, error) {
	tid := encode.GenTID()
	v := make(map[string]interface{})
	v["t"] = fmt.Sprintf("%x", tid)
	v["y"] = "q"
	v["q"] = "find_node"
	args := make(map[string]string)
	args["id"] = encode.ownNode.Info.ID.String()
	args["target"] = target.ID.String()
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

func (encode *KRPC) EncodingNodeResult(tid string, nodes []byte) (string, error) {
	v := make(map[string]interface{})
	v["t"] = fmt.Sprintf("%s", tid)
	v["y"] = "r"
	args := make(map[string]string)
	args["id"] = encode.ownNode.Info.ID.String()
	args["nodes"] = bytes.NewBuffer(nodes).String()
	v["r"] = args
	s, err := bencode.EncodeString(v)
	return s, err
}
