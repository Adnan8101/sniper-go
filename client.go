package main

import (
	"fmt"
	"sync/atomic"
	"time"
)

const keepAliveTick = maxIdleConnDuration / 3

func startConnectionKeepAlive() {
	n := cfg.SalvoSize
	if n <= 0 {
		n = 2
	}
	preWarmConns(n)
	go func() {
		tk := time.NewTicker(keepAliveTick)
		defer tk.Stop()
		for range tk.C {
			n := cfg.SalvoSize
			if n <= 0 {
				n = 2
			}
			preWarmConns(n)

			if fires := atomic.LoadInt64(&fireCount); fires > 0 {
				dials := atomic.LoadInt64(&dialCount)
				fmt.Printf("\x1b[90m[STATS] fires=%d coldDials=%d (%.1f%% cold)\x1b[0m\n",
					fires, dials, 100*float64(dials)/float64(fires))
			}
		}
	}()
}
