package main

import (
	"fmt"
	"github.com/zhujun1980/dhtrobot/dht"
	"os"
)

func main() {
	//reader, _ := os.Open("9d7cdf2a40cc491101641c257e97ce0d6247c848")
	//node := new(dht.Node)
	//r, _ := dht.LoadRouting(node, reader)
	//r.Print()
	//return

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
