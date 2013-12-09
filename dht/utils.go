package dht

import (
	"time"
)

func FanInRequests(rs []*Request, t time.Duration) chan *Request {
	ch := make(chan *Request, len(rs))

	for _, r := range rs {
		go func(r *Request) {
			//r.Node.Log.Printf("Wait response #%s", r.Tid)
			select {
			case s := <-r.ch:
				ch <- s
				r.Node.Log.Printf("Response #%s return", r.Tid)
				return
			case <-time.After(t):
				r.Node.Log.Printf("Wait timeout #%s", r.Tid)
				ch <- nil
				return
			}
		}(r)
	}
	return ch
}
