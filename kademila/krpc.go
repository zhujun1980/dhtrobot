package kademila

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"reflect"
	"strconv"
	"sync"

	"github.com/zeebo/bencode"
)

type RawData struct {
	Addr net.Addr
	Data []byte
}

type PingQuery struct {
	ID string
}

type FindNodeQuery struct {
	ID     string
	Target string
}

type GetPeersQuery struct {
	ID       string
	InfoHash string
}

type AnnouncePeerQuery struct {
	ID          string
	ImpliedPort bool
	InfoHash    string
	Port        int
	Token       string
}

type PingResponse struct {
	ID string
}

type FindNodeResponse struct {
	ID    string
	Nodes []Node
}

type GetPeersResponse struct {
	ID     string
	Token  string
	Values []*Peer
	Nodes  []Node
}

type AnnouncePeerResponse struct {
	ID string
}

type Err struct {
	Code int
	Desc string
}

type Message struct {
	N Node
	//The transaction ID should be encoded as a short string of binary numbers,
	//typically 2 characters are enough as they cover 2^16 outstanding queries.
	T string
	//Every message also has a key "y" with a single character value describing the type of message. [q|r|e]
	Y string
	//The string should be a two character client identifier registered in BEP 20 [3] followed by a two character version identifier.
	//Not all implementations include a "v" key so clients should not assume its presence.
	V string
	Q string
	A interface{}
}

func (q *PingQuery) String() string {
	return fmt.Sprintf("ID=%x", q.ID)
}

func (q *FindNodeQuery) String() string {
	return fmt.Sprintf("ID=%x, Target=%x", q.ID, q.Target)
}

func (q *GetPeersQuery) String() string {
	return fmt.Sprintf("ID=%x, InfoHash=%x", q.ID, q.InfoHash)
}

func (q *AnnouncePeerQuery) String() string {
	return fmt.Sprintf("ID=%x, InfoHash=%x, ImpliedPort=%t, Port=%d, Token=%x", q.ID, q.InfoHash, q.ImpliedPort, q.Port, q.Token)
}

func (r *PingResponse) String() string {
	return fmt.Sprintf("ID=%x", r.ID)
}

func (r *FindNodeResponse) String() string {
	nodes := ""
	for i, n := range r.Nodes {
		s := fmt.Sprintf("<%d, ID=%x, Addr=%s>", i, n.ID, n.Addr.String())
		nodes += s
		if i < len(r.Nodes)-1 {
			nodes += "; "
		}
	}
	return fmt.Sprintf("ID=%x, Length=%d, Nodes=[%s]", r.ID, len(r.Nodes), nodes)
}

func (r *GetPeersResponse) String() string {
	values := ""
	for i, p := range r.Values {
		s := fmt.Sprintf("<%d, Addr=%s:%d>", i, p.IP.String(), p.Port)
		values += s
		if i < len(r.Values)-1 {
			values += "; "
		}
	}
	nodes := ""
	for i, n := range r.Nodes {
		s := fmt.Sprintf("<%d, ID=%x, Addr=%s>", i, n.ID, n.Addr.String())
		nodes += s
		if i < len(r.Nodes)-1 {
			nodes += "; "
		}
	}
	return fmt.Sprintf("ID=%x, Token=%x, Values=[%s], Nodes=[%s]", r.ID, r.Token, values, nodes)
}

func (r *AnnouncePeerResponse) String() string {
	return fmt.Sprintf("ID=%x", r.ID)
}

func (e *Err) String() string {
	return fmt.Sprintf("Code=%d, Desc=%s", e.Code, e.Desc)
}

func (m *Message) String() string {
	var additional string

	if m.Y == "e" {
		e := m.A.(*Err)
		additional = e.String()
	} else {
		switch m.Q {
		case "ping":
			if m.Y == "q" {
				additional = m.A.(*PingQuery).String()
			} else {
				additional = m.A.(*PingResponse).String()
			}
		case "find_node":
			if m.Y == "q" {
				additional = m.A.(*FindNodeQuery).String()
			} else {
				additional = m.A.(*FindNodeResponse).String()
			}
		case "get_peers":
			if m.Y == "q" {
				additional = m.A.(*GetPeersQuery).String()
			} else {
				additional = m.A.(*GetPeersResponse).String()
			}
		case "announce_peer":
			if m.Y == "q" {
				additional = m.A.(*AnnouncePeerQuery).String()
			} else {
				additional = m.A.(*AnnouncePeerResponse).String()
			}
		}
	}
	ver := formatVersion(m.V)
	return fmt.Sprintf("Message T=%x, Y=%s, V=%s, Q=%s, %s, SendNode(%s)",
		m.T, m.Y, ver, m.Q, additional, m.N)
}

type DecodeError struct {
	What string
}

func (e DecodeError) Error() string {
	return fmt.Sprintf("Decode error: %s", e.What)
}

type EncodeError struct {
	What string
}

func (e EncodeError) Error() string {
	return fmt.Sprintf("Encode error: %s", e.What)
}

var GenericError = 201
var ServerError = 202
var ProtocolError = 203
var MethodUnknown = 204

var ErrorDefinitions = map[int]string{
	GenericError:  "Generic Error",
	ServerError:   "Server Error",
	ProtocolError: "Protocol Error",
	MethodUnknown: "Method Unknown",
}

var lock = new(sync.Mutex)
var penddingRequests = make(map[string]string)
var transactionID uint64

func genTID() string {
	transactionID++
	tid := uint16(transactionID % math.MaxUint16)
	bs := make([]byte, 2)
	binary.BigEndian.PutUint16(bs, tid)
	return string(bs)
}

func convertIPPort(buf *bytes.Buffer, ip net.IP, port int) {
	if ip.IsUnspecified() {
		buf.Write(net.ParseIP("127.0.0.1").To4())
	} else {
		buf.Write(ip.To4())
	}
	bs := make([]byte, 2)
	binary.BigEndian.PutUint16(bs, uint16(port))
	buf.Write(bs)
}

func ConvertNodeToBytes(nodes []Node) []byte {
	buf := bytes.NewBuffer(nil)
	for _, v := range nodes {
		buf.Write(v.ID)
		convertIPPort(buf, v.IP(), v.Port())
	}
	return buf.Bytes()
}

func ConvertPeerToBytes(peers []*Peer) []string {
	var ret []string

	for _, v := range peers {
		buf := bytes.NewBuffer(nil)
		convertIPPort(buf, v.IP, v.Port)
		ret = append(ret, string(buf.Bytes()))
	}
	return ret
}

func ParseNodes(sn string) []Node {
	data := []byte(sn)
	var nodes []Node
	for j := 0; j < len(data); j = j + 26 {
		if j+26 > len(data) {
			break
		}
		kn := data[j : j+26]
		ID := NodeID(kn[0:20])
		IP := kn[20:24]
		port := kn[24:26]
		Port := int(port[0])<<8 + int(port[1])
		Addr := &net.UDPAddr{IP: IP, Port: Port}
		nodes = append(nodes, Node{ID: ID, Addr: Addr, Status: INIT})
	}
	return nodes
}

func ParsePeers(peers []string) ([]*Peer, error) {
	var ret []*Peer
	for _, peer := range peers {
		data := []byte(peer)
		if len(data) != 6 {
			return nil, &DecodeError{"Protocol error: peer length would be 6 bytes, got " + strconv.Itoa(len(data))}
		}
		p := new(Peer)
		p.IP = data[:4]
		port := data[4:]
		p.Port = int(port[0])<<8 + int(port[1])
		ret = append(ret, p)
	}
	return ret, nil
}

func formatVersion(ver string) string {
	if len(ver) > 1 {
		v := ""
		if len(ver) == 4 {
			v = fmt.Sprintf("%s(%v.%v)", ver[0:2], ver[2], ver[3])
		} else {
			v = fmt.Sprintf("%s(%x)", ver[0:2], ver[2:])
		}
		return v
	}
	return ver
}

func validateClient(ver string) bool {
	_, found := FilteredClients[formatVersion(ver)]
	return !found
}

func decodeQuery(q string, addition map[string]interface{}, m *Message) error {
	var ok bool
	var emsg string

	switch q {
	case "ping":
		payload := new(PingQuery)
		payload.ID, ok = addition["id"].(string)
		if !ok {
			emsg = "Invalid `id` field"
			break
		}
		m.N.ID = []byte(payload.ID)
		m.A = payload
	case "find_node":
		payload := new(FindNodeQuery)
		payload.ID, ok = addition["id"].(string)
		if !ok {
			emsg = "Invalid `id` field"
			break
		}
		payload.Target, ok = addition["target"].(string)
		if !ok {
			emsg = "Invalid `target` field"
			break
		}
		m.N.ID = []byte(payload.ID)
		m.A = payload
	case "get_peers":
		payload := new(GetPeersQuery)
		payload.ID, ok = addition["id"].(string)
		if !ok {
			emsg = "Invalid `id` field"
			break
		}
		payload.InfoHash, ok = addition["info_hash"].(string)
		if !ok {
			emsg = "Invalid `infohash` field"
			break
		}
		m.N.ID = []byte(payload.ID)
		m.A = payload
	case "announce_peer":
		payload := new(AnnouncePeerQuery)
		payload.ID, ok = addition["id"].(string)
		if !ok {
			emsg = "Invalid `id` field"
			break
		}
		payload.InfoHash, ok = addition["info_hash"].(string)
		if !ok {
			emsg = "Invalid `infohash` field"
			break
		}
		payload.Port, ok = addition["port"].(int)
		if !ok {
			emsg = "Invalid `port` field"
			break
		}
		payload.Token, ok = addition["token"].(string)
		if !ok {
			emsg = "Invalid `token` field"
			break
		}
		var impliedPort int
		impliedPort, ok = addition["implied_port"].(int)
		if !ok {
			impliedPort = 0
			ok = true
		}
		if impliedPort == 1 {
			payload.ImpliedPort = true
		}
		m.N.ID = []byte(payload.ID)
		m.A = payload
	default:
		return &DecodeError{"Unknown method: " + q}
	}
	if !ok {
		return &DecodeError{"Protocol error: " + q + ", detail: " + emsg}
	}
	return nil
}

func decodeResponse(q string, addition map[string]interface{}, m *Message) (interface{}, error) {
	var ok bool
	var ret interface{}
	var err error

	switch q {
	case "ping":
		payload := new(PingResponse)
		payload.ID, ok = addition["id"].(string)
		if !ok {
			break
		}
		m.N.ID = []byte(payload.ID)
		ret = payload
	case "find_node":
		payload := new(FindNodeResponse)
		payload.ID, ok = addition["id"].(string)
		if !ok {
			break
		}
		var sn string
		sn, ok = addition["nodes"].(string)
		if !ok {
			break
		}
		payload.Nodes = ParseNodes(sn)
		m.N.ID = []byte(payload.ID)
		ret = payload
	case "get_peers":
		payload := new(GetPeersResponse)
		payload.ID, ok = addition["id"].(string)
		if !ok {
			break
		}
		payload.Token, ok = addition["token"].(string)
		if !ok {
			break
		}
		var values []string
		values, ok = addition["values"].([]string)
		if ok {
			payload.Values, err = ParsePeers(values)
			if err != nil {
				return nil, err
			}
		}
		var sn string
		sn, ok = addition["nodes"].(string)
		if ok {
			payload.Nodes = ParseNodes(sn)
		}
		if len(payload.Values) == 0 && len(payload.Nodes) == 0 {
			ok = false
			break
		}
		m.N.ID = []byte(payload.ID)
		ret = payload
	case "announce_peer":
		payload := new(AnnouncePeerResponse)
		payload.ID, ok = addition["id"].(string)
		if !ok {
			break
		}
		m.N.ID = []byte(payload.ID)
		ret = payload
	}
	if !ok {
		return nil, &DecodeError{"Protocol error: " + q}
	}
	return ret, nil
}

func KRPCValidate(data []byte) bool {
	val := make(map[string]interface{})
	err := bencode.DecodeBytes(data, &val)
	return (err == nil)
}

func KRPCDecode(raw *RawData) (*Message, error) {
	val := make(map[string]interface{})
	var ok bool
	var err error

	if err = bencode.DecodeBytes(raw.Data, &val); err != nil {
		return nil, &DecodeError{fmt.Sprintf("bencode error: %s, %s, %d", err.Error(), string(raw.Data), len(raw.Data))}
	}

	m := new(Message)
	m.N.Addr = raw.Addr
	m.T, ok = val["t"].(string)
	if !ok {
		return nil, &DecodeError{"Protocol error: Empty `t` field"}
	}
	m.Y, ok = val["y"].(string)
	if !ok {
		return nil, &DecodeError{"Protocol error: Empty `y` field"}
	}
	m.V, ok = val["v"].(string)
	if !ok {
		m.V = ""
	}
	switch m.Y {
	case "q":
		m.Q, ok = val["q"].(string)
		if !ok {
			return nil, &DecodeError{"Protocol error: Empty query `q` field"}
		}
		var addition map[string]interface{}
		addition, ok = val["a"].(map[string]interface{})
		if !ok {
			return nil, &DecodeError{"Protocol error: Empty query `a` field"}
		}
		err = decodeQuery(m.Q, addition, m)
		if err != nil {
			return nil, err
		}
	case "r":
		lock.Lock()
		defer lock.Unlock()
		m.Q, ok = penddingRequests[m.T]
		if !ok {
			return nil, &DecodeError{"Unknown request"}
		}
		var addition map[string]interface{}
		addition, ok = val["r"].(map[string]interface{})
		if !ok {
			return nil, &DecodeError{"Protocol error: Empty response `r` field"}
		}
		m.A, err = decodeResponse(m.Q, addition, m)
		if err != nil {
			return nil, err
		}
	case "e":
		lock.Lock()
		defer lock.Unlock()
		m.Q, ok = penddingRequests[m.T]
		if !ok {
			return nil, &DecodeError{"Unknown request"}
		}
		err := new(Err)
		var decodeErr []interface{}
		decodeErr, ok = val["e"].([]interface{})
		if !ok {
			return nil, &DecodeError{"Protocol error: Empty error `e` field"}
		}
		if len(decodeErr) != 2 {
			return nil, &DecodeError{"Protocol error: Error field length would be 2, got " + strconv.Itoa(len(decodeErr))}
		}
		err.Code, ok = decodeErr[0].(int)
		if !ok {
			c, ok := decodeErr[0].(int64)
			if !ok {
				return nil, &DecodeError{fmt.Sprintf("Protocol error: Invalid Error code type, dat = %v, %s, %s", decodeErr, reflect.TypeOf(decodeErr[0]), reflect.TypeOf(decodeErr[1]))}
			}
			err.Code = int(c)
		}
		err.Desc, ok = decodeErr[1].(string)
		if !ok {
			return nil, &DecodeError{"Protocol error: Invalid Error description type"}
		}
		m.A = err
	default:
		return nil, &DecodeError{"Protocol error: Unknown `y` value " + m.Y}
	}

	return m, nil
}

func KRPCNewError(tid string, q string, code int) *Message {
	m := new(Message)
	m.T = tid
	m.Y = "e"
	m.Q = q
	payload := new(Err)
	payload.Code = code
	payload.Desc = ErrorDefinitions[code]
	m.A = payload
	return m
}

func KRPCNewPing(local NodeID) *Message {
	m := new(Message)
	m.Y = "q"
	m.Q = "ping"
	payload := new(PingQuery)
	payload.ID = local.String()
	m.A = payload
	return m
}

func KRPCNewPingResponse(tid string, local NodeID) *Message {
	m := new(Message)
	m.T = tid
	m.Y = "r"
	m.Q = "ping"
	payload := new(PingResponse)
	payload.ID = local.String()
	m.A = payload
	return m
}

func KRPCNewFindNode(local NodeID, target NodeID) *Message {
	m := new(Message)
	m.Y = "q"
	m.Q = "find_node"
	payload := new(FindNodeQuery)
	payload.ID = local.String()
	payload.Target = target.String()
	m.A = payload
	return m
}

func KRPCNewFindNodeResponse(tid string, local NodeID, nodes []Node) *Message {
	m := new(Message)
	m.T = tid
	m.Y = "r"
	m.Q = "find_node"
	payload := new(FindNodeResponse)
	payload.ID = local.String()
	payload.Nodes = nodes
	m.A = payload
	return m
}

func KRPCNewGetPeers(local NodeID, infoHash NodeID) *Message {
	m := new(Message)
	m.Y = "q"
	m.Q = "get_peers"
	payload := new(GetPeersQuery)
	payload.ID = local.String()
	payload.InfoHash = infoHash.String()
	m.A = payload
	return m
}

func KRPCNewGetPeersResponse(tid string, local NodeID, token string, nodes []Node, values []*Peer) *Message {
	m := new(Message)
	m.T = tid
	m.Y = "r"
	m.Q = "get_peers"
	payload := new(GetPeersResponse)
	payload.ID = local.String()
	payload.Nodes = nodes
	payload.Values = values
	payload.Token = token
	m.A = payload
	return m
}

func KRPCNewAnnouncePeer(local NodeID, infoHash string, port int, token string, impliedPort bool) *Message {
	m := new(Message)
	m.Y = "q"
	m.Q = "announce_peer"
	payload := new(AnnouncePeerQuery)
	payload.ID = local.String()
	payload.InfoHash = infoHash
	payload.Port = port
	payload.Token = token
	payload.ImpliedPort = impliedPort
	m.A = payload
	return m
}

func KRPCEncode(m *Message) (string, error) {
	var ret string
	var err error

	switch m.Y {
	case "q":
		tid := genTID()

		switch m.Q {
		case "ping":
			a := m.A.(*PingQuery)
			ret, err = KRPCEncodePing(tid, a.ID)
		case "find_node":
			a := m.A.(*FindNodeQuery)
			ret, err = KRPCEncodeFindNode(tid, a.ID, a.Target)
		case "get_peers":
			a := m.A.(*GetPeersQuery)
			ret, err = KRPCEncodeGetPeers(tid, a.ID, a.InfoHash)
		case "announce_peer":
			a := m.A.(*AnnouncePeerQuery)
			ret, err = KRPCEncodeAnnouncePeer(tid, a.ID, a.InfoHash, a.Port, a.Token, a.ImpliedPort)
		}
		if err == nil {
			lock.Lock()
			defer lock.Unlock()
			penddingRequests[tid] = m.Q
		}
		m.T = tid
	case "r":
		switch m.Q {
		case "ping":
			a := m.A.(*PingResponse)
			ret, err = KRPCEncodePingResponse(m.T, a.ID)
		case "find_node":
			a := m.A.(*FindNodeResponse)
			ret, err = KRPCEncodeFindNodeResponse(m.T, a.ID, a.Nodes)
		case "get_peers":
			a := m.A.(*GetPeersResponse)
			ret, err = KRPCEncodeGetPeersResponse(m.T, a.ID, a.Token, a.Nodes, a.Values)
		case "announce_peer":
		}

	case "e":
		a := m.A.(*Err)
		ret, err = KRPCEncodeError(m.T, a.Code, a.Desc)
	}
	return ret, err
}

func KRPCEncodeError(tid string, code int, desc string) (string, error) {
	v := make(map[string]interface{})
	v["t"] = tid
	v["y"] = "e"
	e := make([]interface{}, 2)
	e[0] = code
	e[1] = desc
	v["e"] = e
	s, err := bencode.EncodeString(v)
	return s, err
}

func KRPCEncodePing(tid string, local string) (string, error) {
	v := make(map[string]interface{})
	v["t"] = tid
	v["y"] = "q"
	v["q"] = "ping"
	v["a"] = map[string]string{
		"id": local,
	}
	s, err := bencode.EncodeString(v)
	return s, err
}

func KRPCEncodePingResponse(tid string, local string) (string, error) {
	v := make(map[string]interface{})
	v["t"] = tid
	v["y"] = "r"
	v["r"] = map[string]string{
		"id": local,
	}
	s, err := bencode.EncodeString(v)
	return s, err
}

func KRPCEncodeFindNode(tid string, local string, target string) (string, error) {
	v := make(map[string]interface{})
	v["t"] = tid
	v["y"] = "q"
	v["q"] = "find_node"
	v["a"] = map[string]string{
		"id":     local,
		"target": target,
	}
	s, err := bencode.EncodeString(v)
	return s, err
}

func KRPCEncodeFindNodeResponse(tid string, local string, nodes []Node) (string, error) {
	v := make(map[string]interface{})
	v["t"] = tid
	v["y"] = "r"
	v["r"] = map[string]string{
		"id":    local,
		"nodes": string(ConvertNodeToBytes(nodes)),
	}
	s, err := bencode.EncodeString(v)
	return s, err
}

func KRPCEncodeGetPeers(tid string, local string, infoHash string) (string, error) {
	v := make(map[string]interface{})
	v["t"] = tid
	v["y"] = "q"
	v["q"] = "get_peers"
	v["a"] = map[string]string{
		"id":        local,
		"info_hash": infoHash,
	}
	s, err := bencode.EncodeString(v)
	return s, err
}

func KRPCEncodeGetPeersResponse(tid string, local string, token string, nodes []Node, values []*Peer) (string, error) {
	v := make(map[string]interface{})
	v["t"] = tid
	v["y"] = "r"
	v["r"] = map[string]interface{}{
		"id":     local,
		"token":  token,
		"nodes":  ConvertNodeToBytes(nodes),
		"values": ConvertPeerToBytes(values),
	}
	s, err := bencode.EncodeString(v)
	return s, err
}

func KRPCEncodeAnnouncePeer(tid string, local string, infoHash string, port int, token string, impliedPort bool) (string, error) {
	v := make(map[string]interface{})
	v["t"] = tid
	v["y"] = "q"
	v["q"] = "announce_peer"

	imp := 0
	if impliedPort {
		imp = 1
	}
	v["a"] = map[string]interface{}{
		"id":           local,
		"implied_port": imp,
		"info_hash":    infoHash,
		"port":         port,
		"token":        token,
	}
	s, err := bencode.EncodeString(v)
	return s, err
}

func KRPCEncodeAnnouncePeerResponse(tid string, local string) (string, error) {
	v := make(map[string]interface{})
	v["t"] = tid
	v["y"] = "r"
	v["r"] = map[string]string{
		"id": local,
	}
	s, err := bencode.EncodeString(v)
	return s, err
}
