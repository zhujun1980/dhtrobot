package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/zhujun1980/dhtrobot/kademila"
)

/*
TODO
	NAT
	Libtorrent 不使用 target 和 infohash
	routing table 刷新
	routing table 保存
*/

func initLogger() *logrus.Logger {
	var log = logrus.New()
	log.Formatter = &logrus.TextFormatter{
		FullTimestamp: true,
	}
	log.Out = os.Stderr
	log.Level = logrus.DebugLevel
	return log
}

var (
	flagClient bool
)

func parseCommandLine() {
	flag.BoolVar(&flagClient, "client", false, "Run program in client mode")
	flag.Parse()
}

func main() {
	parseCommandLine()

	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	var logger = initLogger()
	master := make(chan string)

	if flagClient {
		kademila.RunClient(ctx, master, logger)
	} else {
		dht, _ := kademila.New(ctx, master, logger)
		for {
			select {
			case msg := <-master:
				fmt.Println(msg)
			case <-ctx.Done():
				dht.Close()
				return
			}
		}
	}
}
