package dht

import (
	"bytes"
	"crypto/sha1"
	"io"
	"time"
)

type TokenVal struct {
	node   *NodeInfo
	create time.Time
}

func (node *Node) GenToken(sender *NodeInfo) string {
	hash := sha1.New()
	io.WriteString(hash, sender.IP.String())
	io.WriteString(hash, time.Now().String())
	io.WriteString(hash, node.secret)
	token := bytes.NewBuffer(hash.Sum(nil)).String()
	tv := new(TokenVal)
	tv.create = time.Now()
	tv.node = sender
	node.tokens[token] = tv

	return token
}
