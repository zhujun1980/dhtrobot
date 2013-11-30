package dht

import (
	"fmt"
	"github.com/zeebo/bencode"
	"math"
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

type Response struct {
	R map[string]interface{}
}

type Err struct {
	E []interface{}
}

type KRPCMessage struct {
	T      string
	Y      string
	Addion interface{}
}

func (encode *KRPC) GenTID() uint32 {
	return encode.autoID() % math.MaxUint16
}

func (encode *KRPC) autoID() uint32 {
	return atomic.AddUint32(&encode.tid, 1)
}

func (encode *KRPC) DecodeFindNodes(msg string) []*NodeInfo {
	message, err := encode.Decode(msg)
	if err != nil {
		encode.ownNode.Log.Panicln(err)
		return nil
	}
	res := message.Addion.(*Response)
	nodes := []byte(res.R["nodes"].(string))
	rets := ParseBytesStream(nodes)
	return rets
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

func (encode *KRPC) Decode(message string) (*KRPCMessage, error) {
	val := make(map[string]interface{})

	if err := bencode.DecodeString(message, &val); err != nil {
		return nil, err
	} else {
		message := new(KRPCMessage)
		message.T = val["t"].(string)
		message.Y = val["y"].(string)
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

func (encode *KRPC) FindNode(target *NodeInfo) (uint32, string, error) {
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
