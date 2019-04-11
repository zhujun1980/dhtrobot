package kademila

const APPNAME = "DHTRobot"

const VERSION = "0.1.0"

// K is the bucket's size
const K int = 8

// BOOTSTRAP is bootstrap node
var BOOTSTRAP = []string{
	"router.bittorrent.com:6881",
	"dht.transmissionbt.com:6881",
	"service.ygrek.org.ua:6881",
	"router.utorrent.com:6881",
	"router.transmission.com:6881",
}

const MAXSIZE = 2048

const (
	INIT         = iota
	GOOD         = iota
	QUESTIONABLE = iota
	BAD          = iota
)

const (
	Running  = iota
	Suspend  = iota
	Finished = iota
)

const FindNodeTimeLimit = 120 // In seconds

const MaxUnchangedCount = 5000

const TokenTimeLimit = 300 // 5 minutes

const NodeRefreshnessTimeLimit = 15

const BucketLastChangedTimeLimit = 15

const MaxBitsLength = 160

var FilteredClients = map[string]bool{
	"LT(0.17)": true,
}
