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
	if len(ids) > 0 {
		for _, id := range ids {
			go func() {
				node := dht.NewNode(dht.HexToID(id), logger, master)
				node.Run()
			}()
		}
	} else {
		for i := 0; i < dht.NODENUM; i++ {
			go func() {
				node := dht.NewNode(dht.GenerateID(), logger, master)
				node.Run()
			}()
		}
	}

	for {
		select {
		case msg := <-master:
			fmt.Println(msg)
		}
	}
}
