package kademila

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/binary"
	"math/rand"
	"time"
)

type TokenBuilder struct {
	token      uint32
	old        uint32
	lastUpdate time.Time
}

func newTokenBuilder() *TokenBuilder {
	tk := new(TokenBuilder)
	rand.Seed(time.Now().Unix())
	tk.token = rand.Uint32()
	tk.old = tk.token
	tk.lastUpdate = time.Now()
	return tk
}

func (tk *TokenBuilder) renewToken() {
	now := time.Now()
	diff := now.Sub(tk.lastUpdate)
	if diff.Seconds() >= TokenTimeLimit {
		tk.old = tk.token
		tk.token = rand.Uint32()
		tk.lastUpdate = time.Now()
	}
}

func (tk *TokenBuilder) create(ip string) string {
	return string(tk.createHMac(ip, tk.token))
}

func (tk *TokenBuilder) createHMac(msg string, key uint32) []byte {
	bs := make([]byte, 4)
	binary.BigEndian.PutUint32(bs, key)
	mac := hmac.New(sha1.New, bs)
	mac.Write([]byte(msg))
	return mac.Sum(nil)
}

func (tk *TokenBuilder) validate(cryptoed string, ip string) bool {
	if hmac.Equal([]byte(cryptoed), tk.createHMac(ip, tk.token)) ||
		hmac.Equal([]byte(cryptoed), tk.createHMac(ip, tk.old)) {
		return true
	}
	return false
}
