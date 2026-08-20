package main

import (
	"runtime"
	"time"
)

func startConnectionKeepAlive() {
	n := cfg.SalvoSize
	if n <= 0 {
		n = 2
	}
	preWarmConns(n)
	go func() {
		tk := time.NewTicker(3 * time.Second)
		for range tk.C {
			preWarmConns(1)
			runtime.GC()
		}
	}()
}
