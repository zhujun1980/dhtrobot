package main

import (
	"fmt"
	"github.com/zhujun1980/dhtrobot/dht"
	"os"
)

func main() {
	persist := dht.GetPersist()
	if persist == nil {
		fmt.Println("db err")
		return
	}
	ids, err := persist.LoadAllNodeIDs()
	if err != nil {
		panic(err)
		return
	}
	if len(ids) > 0 {
		for _, id := range ids {
			node, err := persist.LoadNodeInfo(id, os.Stdout)
			if err != nil {
				panic(err)
				return
			}
			node.Run()
		}
	} else {
		for i := 0; i < dht.NODENUM; i++ {
			go func() {
				node := dht.NewNode(os.Stdout)
				node.Run()
			}()
		}
	}

	ch := make(chan bool)
	<-ch
}
