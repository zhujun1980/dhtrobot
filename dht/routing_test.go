package dht

import (
	"os"
	"testing"
	"time"
)

func NodeObj() *Node {
	n := NewNode(GenerateID(), os.Stdout, nil)
	return n
}

func TestRoutingLen(t *testing.T) {
	n := NodeObj()
	if n.Routing.Len() != 0 {
		t.Error("len isn't zero")
	}
	n.Routing.InsertNode(&NodeInfo{nil, 0, GenerateID(), GOOD, time.Now()})
	if n.Routing.Len() != 1 {
		t.Error("len err, want 1")
	}
}
