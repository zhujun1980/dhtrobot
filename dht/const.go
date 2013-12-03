package dht

import (
	"time"
)

const K int = 8

const ALPHA int = 3

const (
	NEW    = iota
	SENIOR = iota
)

const (
	TRANSMISSIONBT = "dht.transmissionbt.com:6881"
)

const MAXSIZE = 500

const (
	GOOD           = iota
	QUESTIONABLE_1 = iota //MISS once
	QUESTIONABLE_2 = iota //MISS twice
	BAD            = iota
)

//username:password@protocol(address)/dbname?param=value
const DSN = "root:123456@tcp(localhost:3306)/dhtrobot?charset=utf8"

const (
	NODENUM = 1
)

const (
	EXPIRE_DURATION = time.Minute * 15
)
