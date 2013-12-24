package main

import (
	"flag"
	"github.com/zhujun1980/dhtrobot/dht"
	"net"
	"os"
)

var cmd string
var infohash string
var ip string
var port int

func init() {
	flag.StringVar(&cmd, "cmd", "", "krpc command")
	flag.StringVar(&infohash, "infohash", "", "infohash")
	flag.StringVar(&ip, "h", "127.0.0.1", "ip")
	flag.IntVar(&port, "p", 0, "port")
}

func main() {
	flag.Parse()

	logger := os.Stdout
	mlogger := os.Stdout
	master := make(chan string)
	node := dht.NewNode(dht.GenerateID(), logger, mlogger, master)
	node.Cli()
	switch cmd {
	case "ping":
		raddr := &net.UDPAddr{net.ParseIP(ip), port, ""}
		node.Ping(raddr)
		break
	case "find_node":
		raddr := &net.UDPAddr{net.ParseIP(ip), port, ""}
		node.FindNode(raddr, node.ID())
		break
	case "get_peers":
		raddr := &net.UDPAddr{net.ParseIP(ip), port, ""}
		node.GetPeersAndAnnounce(raddr, dht.HexToID(infohash))
		break
	}
}










