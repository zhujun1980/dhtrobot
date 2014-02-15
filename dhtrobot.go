package main

import (
	"fmt"
	"github.com/zhujun1980/dhtrobot/dht"
	"os"
)

func main() {
	persist := dht.GetPersist()
	ids, err := persist.LoadAllNodeIDs()
	if err != nil {
		panic(err)
		return
	}

	master := make(chan string)
	logger := os.Stdout
	mlogger, err := os.OpenFile("msg", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0744)
	if err != nil {
		panic(err)
		return
	}
	if len(ids) > 0 {
		for _, id := range ids {
			go func(curid string) {
				node := dht.NewNode(dht.HexToID(curid), logger, mlogger, master)
				node.Run()
			}(id)
		}
	}

	for i := 0; i < dht.NODENUM-len(ids); i++ {
		go func() {
			node := dht.NewNode(dht.GenerateID(), logger, mlogger, master)
			node.Run()
		}()
	}

	for {
		select {
		case msg := <-master:
			fmt.Println(msg)
		}
	}
}
