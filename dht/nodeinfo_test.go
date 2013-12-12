package dht

import (
	"testing"
)

func TestRandID(t *testing.T) {
	src := GenerateID()
	dst := RandID(src, 150)
	if BucketIndex(src, dst) != 150 {
		t.Error("RandID error")
	}
}
