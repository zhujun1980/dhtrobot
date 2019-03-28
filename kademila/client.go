package kademila

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	colorHeader  = "\033[95m"
	colorOkBlue  = "\033[94m"
	colorOkGreen = "\033[92m"
	colorWarning = "\033[93m"
	colorFail    = "\033[91m"
	colorDim     = "\033[2m"
	colorEndC    = "\033[0m"
)

const (
	tkCmdDistance = iota
	tkCmdConnnect = iota
	tkCmdPing     = iota
	tkCmdFindNode = iota
	tkCmdGetPeers = iota
	tkCmdListConn = iota
	tkCmdShowInfo = iota
	tkCmdExit     = iota
	tkCmdHelp     = iota
	tkNUMBER      = iota
	tkNODEID      = iota
	tkID          = iota
	tkSTRING      = iota
	tkConnection  = iota
)

var tkNames = []string{
	"tkCmdDistance", "tkCmdConnnect", "tkCmdPing", "tkCmdFindNode",
	"tkCmdGetPeers", "tkCmdListConn", "tkCmdShowInfo", "tkCmdExit",
	"tkCmdHelp", "tkNUMBER", "tkNODEID", "tkID", "tkSTRING", "tkConnection",
}

type connection struct {
	conn *net.UDPConn
	addr *net.UDPAddr
}

func (c *connection) close() {
	c.conn.Close()
}

var allConnections = map[string]*connection{}

var infos = map[string]string{}

type token struct {
	tk    int
	value string
}

func (t *token) String() string {
	return fmt.Sprintf("%s(%d): %s", tkNames[t.tk], t.tk, t.value)
}

var distancePattern = []int{tkCmdDistance, tkNODEID, tkNODEID}
var connectPattern = []int{tkCmdConnnect, tkID, tkSTRING, tkNUMBER}
var connect2Pattern = []int{tkCmdConnnect, tkConnection, tkSTRING, tkNUMBER}
var pingPattern = []int{tkCmdPing, tkConnection}
var findNodePattern = []int{tkCmdFindNode, tkConnection, tkNODEID}
var getPeersPattern = []int{tkCmdGetPeers, tkConnection, tkNODEID}
var listPattern = []int{tkCmdListConn}
var infoPattern = []int{tkCmdShowInfo}
var info2Pattern = []int{tkCmdShowInfo, tkSTRING}
var exitPattern = []int{tkCmdExit}
var helpPattern = []int{tkCmdHelp}

type pattern struct {
	p  []int
	op func(ctx context.Context, tokens []*token) error
}

func distance(ctx context.Context, n1 string, n2 string) error {
	c, _ := FromContext(ctx)
	d := Distance(HexToID(n1), HexToID(n2))
	io.WriteString(c.Writer, fmt.Sprintf("%d\n", d))
	return nil
}

func connect(ctx context.Context, name string, host string, port int) error {
	c, _ := FromContext(ctx)

	address := fmt.Sprintf("%s:%d", host, port)
	raddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return err
	}
	io.WriteString(c.Writer, fmt.Sprintf("Connect to %s\n", raddr))
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return err
	}
	_, ok := allConnections[name]
	if ok {
		allConnections[name].close()
		delete(allConnections, name)
	}
	allConnections[name] = &connection{conn, raddr}
	return listConnections(ctx)
}

func listConnections(ctx context.Context) error {
	c, _ := FromContext(ctx)

	i := 1
	for k, v := range allConnections {
		line := fmt.Sprintf("%d. %s, %s -> %s\n", i, k, v.conn.LocalAddr(), v.addr)
		io.WriteString(c.Writer, line)
		i++
	}
	return nil
}

func send(ctx context.Context, conn *connection, req *Message, t string) (*Message, error) {
	c, _ := FromContext(ctx)
	encoded, err := KRPCEncode(req)
	if err != nil {
		return nil, err
	}
	n, err := conn.conn.Write([]byte(encoded))
	if err != nil {
		return nil, err
	}
	io.WriteString(c.Writer, fmt.Sprintf("%s--> %s %s\n", colorHeader, req.String(), colorEndC))
	io.WriteString(c.Writer, fmt.Sprintf("%s--> Packet-Sent: bytes=%d, waiting for response (%s timeout) %s\n", colorHeader, n, t, colorEndC))
	timeout, _ := time.ParseDuration(t)
	deadline := time.Now().Add(timeout)
	err = conn.conn.SetReadDeadline(deadline)
	if err != nil {
		return nil, err
	}
	buffer := make([]byte, MAXSIZE)
	n, addr, err := conn.conn.ReadFrom(buffer)
	if err != nil {
		return nil, err
	}
	io.WriteString(c.Writer, fmt.Sprintf("%s<-- Packet-received: bytes=%d from=%s %s\n", colorOkGreen, n, addr.String(), colorEndC))
	res, err := KRPCDecode(&RawData{addr, buffer[:n]})
	if err != nil {
		return nil, err
	}
	io.WriteString(c.Writer, fmt.Sprintf("%s<-- %s %s\n", colorOkGreen, res.String(), colorEndC))
	return res, nil
}

func ping(ctx context.Context, conn *connection) error {
	c, _ := FromContext(ctx)
	req := KRPCNewPing(c.Local.ID)
	_, err := send(ctx, conn, req, "5s")
	return err
}

func findNode(ctx context.Context, conn *connection, target NodeID) error {
	c, _ := FromContext(ctx)
	req := KRPCNewFindNode(c.Local.ID, target)
	res, err := send(ctx, conn, req, "5s")
	if err != nil {
		return err
	}
	r := res.A.(*FindNodeResponse)
	io.WriteString(c.Writer, fmt.Sprintf("%d nodes received\n", len(r.Nodes)))
	for i := range r.Nodes {
		io.WriteString(c.Writer, fmt.Sprintf("%d %s, Distance=%d, Distance=%d\n", i, r.Nodes[i], Distance(target, r.Nodes[i].ID), Distance(c.Local.ID, r.Nodes[i].ID)))
	}
	return nil
}

func getPeers(ctx context.Context, conn *connection, infoHash NodeID) error {
	c, _ := FromContext(ctx)
	req := KRPCNewGetPeers(c.Local.ID, infoHash)
	res, err := send(ctx, conn, req, "5s")
	if err != nil {
		return err
	}
	r := res.A.(*GetPeersResponse)
	io.WriteString(c.Writer, fmt.Sprintf("%d peers received, %d nodes received\n", len(r.Values), len(r.Nodes)))
	if len(r.Nodes) > 0 {
		io.WriteString(c.Writer, "Node list:\n")
		for i := range r.Nodes {
			io.WriteString(c.Writer, fmt.Sprintf("%d %s, Distance=%d, Distance=%d\n", i, r.Nodes[i], Distance(infoHash, r.Nodes[i].ID), Distance(c.Local.ID, r.Nodes[i].ID)))
		}
	}
	if len(r.Values) > 0 {
		io.WriteString(c.Writer, "Peer list:\n")
		for i := range r.Values {
			io.WriteString(c.Writer, fmt.Sprintf("%d %s\n", i, r.Values[i]))
		}
	}
	return nil
}

func showInfo(ctx context.Context, item string) error {
	c, _ := FromContext(ctx)
	if item != "" {
		v, ok := infos[item]
		if ok {
			io.WriteString(c.Writer, fmt.Sprintf("%s: %s\n", item, v))
		} else {
			io.WriteString(c.Writer, fmt.Sprintf("%s: %s\n", item, ""))
		}
	} else {
		for k, v := range infos {
			io.WriteString(c.Writer, fmt.Sprintf("%s: %s\n", k, v))
		}
	}
	return nil
}

func exit(ctx context.Context) error {
	c, _ := FromContext(ctx)
	io.WriteString(c.Writer, "Bye.\n")
	os.Exit(0)
	return nil
}

func printHelp(ctx context.Context) error {
	c, _ := FromContext(ctx)
	help := `
DHTRobot Usage:
    distance <id1> <id2>:                 show the distance of id1 and id2
    connect <conn_name> <host> <port>:    create a connection to host:port named 'conn_name'
    ping <conn_name>:                     send a 'ping' message to the connection 'conn_name'
    find <conn_name> <id>:                send a 'find_node' message about id to the connection 'conn_name'
    get <conn_name> <infohash>:           send a 'get_peers' message about infohash to the connection 'conn_name'
    list:                                 list all connnections
    info [key]:                           show info
    exit:                                 exit the program
    help:                                 show this help
`
	io.WriteString(c.Writer, help)
	return nil
}

var patterns = []pattern{
	{distancePattern, func(ctx context.Context, tokens []*token) error {
		return distance(ctx, tokens[1].value, tokens[2].value)
	}},
	{connectPattern, func(ctx context.Context, tokens []*token) error {
		port, err := strconv.Atoi(tokens[3].value)
		if err != nil {
			return err
		}
		return connect(ctx, tokens[1].value, tokens[2].value, port)
	}},
	{connect2Pattern, func(ctx context.Context, tokens []*token) error {
		port, err := strconv.Atoi(tokens[3].value)
		if err != nil {
			return err
		}
		return connect(ctx, tokens[1].value, tokens[2].value, port)
	}},
	{pingPattern, func(ctx context.Context, tokens []*token) error { return ping(ctx, allConnections[tokens[1].value]) }},
	{findNodePattern, func(ctx context.Context, tokens []*token) error {
		return findNode(ctx, allConnections[tokens[1].value], HexToID(tokens[2].value))
	}},
	{getPeersPattern, func(ctx context.Context, tokens []*token) error {
		return getPeers(ctx, allConnections[tokens[1].value], HexToID(tokens[2].value))
	}},
	{listPattern, func(ctx context.Context, tokens []*token) error { return listConnections(ctx) }},
	{infoPattern, func(ctx context.Context, tokens []*token) error { return showInfo(ctx, "") }},
	{info2Pattern, func(ctx context.Context, tokens []*token) error { return showInfo(ctx, tokens[1].value) }},
	{exitPattern, func(ctx context.Context, tokens []*token) error { return exit(ctx) }},
	{helpPattern, func(ctx context.Context, tokens []*token) error { return printHelp(ctx) }},
}

type lexer struct {
	expr  string
	tk    int
	regex *regexp.Regexp
}

var lexers = []lexer{
	{"^distance$", tkCmdDistance, nil},
	{"^connect$", tkCmdConnnect, nil},
	{"^ping$", tkCmdPing, nil},
	{"^find$", tkCmdFindNode, nil},
	{"^get$", tkCmdGetPeers, nil},
	{"^list$", tkCmdListConn, nil},
	{"^info$", tkCmdShowInfo, nil},
	{"^exit$", tkCmdExit, nil},
	{"^help$", tkCmdHelp, nil},
	{"^[0-9]+$", tkNUMBER, nil},
	{"^[0-9A-Fa-f]{40}$", tkNODEID, nil},
	{"^[A-Za-z_][[:word:]]*$", tkID, nil},
	{"^[^\t\n\f\r ]+$", tkSTRING, nil},
}

type ParseError struct {
	What string
}

func (e ParseError) Error() string {
	return fmt.Sprintf("Parse error %s", e.What)
}

func getToken(input string) (*token, error) {
	n := new(token)
	n.value = input

	_, ok := allConnections[input]
	if ok {
		n.tk = tkConnection
		return n, nil
	}
	for i := range lexers {
		if lexers[i].regex.MatchString(input) {
			n.tk = lexers[i].tk
			return n, nil
		}
	}
	return nil, ParseError{input}
}

func eval(ctx context.Context, input string) error {
	var tokens []*token

	sc := bufio.NewScanner(strings.NewReader(input))
	sc.Split(bufio.ScanWords)
	for sc.Scan() {
		tk, err := getToken(sc.Text())
		if err != nil {
			return err
		}
		tokens = append(tokens, tk)
	}
	for i := range patterns {
		notmatch := false
		if len(patterns[i].p) != len(tokens) {
			continue
		}
		for j := 0; j < len(tokens); j++ {
			if tokens[j].tk != patterns[i].p[j] {
				notmatch = true
				break
			}
		}
		if !notmatch {
			err := patterns[i].op(ctx, tokens)
			return err
		}
	}
	return ParseError{"No match pattern"}
}

func RunClient(ctx context.Context, master chan string, logger *logrus.Logger) {
	var err error

	logger.Formatter = &logrus.TextFormatter{}

	for i := range lexers {
		lexers[i].regex, err = regexp.Compile(lexers[i].expr)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"err":   err,
				"regex": lexers[i].expr,
			}).Fatal("Compile regex failed")
		}
	}
	c := new(NodeContext)
	c.Log = logger
	c.Master = master
	c.Local.ID = GenerateID()
	c.Writer = os.Stdout

	clientCtx := NewContext(ctx, c)
	io.WriteString(c.Writer, fmt.Sprintf("DHTRobot %s, Type 'help' show help page\n", VERSION))
	io.WriteString(c.Writer, fmt.Sprintf("Local node ID: %s\n", c.Local.ID.HexString()))

	infos["NodeID"] = c.Local.ID.HexString()

	for {
		line, ok := GNUReadLine(">>> ")
		if !ok {
			io.WriteString(c.Writer, "\nBye\n")
			break
		}
		if line == "" {
			continue
		}
		GNUAddHistory(line)
		err = eval(clientCtx, line)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"err":   err,
				"input": line,
			}).Error("Parse command failed")
		}
		io.WriteString(c.Writer, "\n")
	}
}
