package dht

import (
	"math/big"
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

func binew(x int64) *big.Int {
	return big.NewInt(x)
}

func id2bi(id Identifier) *big.Int {
	return big.NewInt(0).SetBytes(id)
}

func birsh(x int64, n uint) *big.Int {
	return big.NewInt(0).Rsh(binew(x), n)
}

func bilsh(x int64, n uint) *big.Int {
	return big.NewInt(0).Lsh(binew(x), n)
}

func bisub(x, y *big.Int) *big.Int {
	return big.NewInt(0).Sub(x, y)
}

func bidiv(x, y *big.Int) *big.Int {
	return big.NewInt(0).Div(x, y)
}

func bimid(max, min *big.Int) *big.Int {
	d := bisub(max, min)
	return d.Rsh(d, 1).Add(d, min)
}

func biadd(x, y *big.Int) *big.Int {
	return big.NewInt(0).Add(x, y)
}










