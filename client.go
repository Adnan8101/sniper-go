package main

import (
	"time"
)

func startConnectionKeepAlive() {
	n := cfg.SalvoSize
	if n <= 0 {
		n = 2
	}
	preWarmConns(n)
	go func() {
		tk := time.NewTicker(30 * time.Second)
		for range tk.C {
			preWarmConns(1)
		}
	}()
}
