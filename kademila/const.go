package kademila

const APPNAME = "DHTRobot"

const VERSION = "0.1.0"

// Bucket size
const K int = 8

// Bootstrap nodes
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

var statusNames = []string{
	"Init", "Good", "Questionable", "Bad",
}

const (
	Running  = iota
	Suspend  = iota
	Finished = iota
)

const RequestTimeout = 10 // seconds

const PingNodeTimeLimit = 30 // seconds

const FindNodeTimeLimit = 120 // seconds

const MaxUnchangedCount = 5000

const TokenTimeLimit = 300 // seconds

const BucketLastChangedTimeLimit = 15 // minutes

const NodeRefreshnessTimeLimit = 60 // seconds

const MaxBitsLength = 160

const FinderNum = 2

const UndefinedWorker = -1

var FilteredClients = map[string]bool{
	"LT(0.17)": true,
}
