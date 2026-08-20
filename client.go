package main
import (
	"runtime"
	"time"
)
func startConnectionKeepAlive() {
	salvo := config.SalvoSize
	if salvo <= 0 {
		salvo = 2
	}
	preWarmConns(salvo)
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		for range ticker.C {
			preWarmConns(1)
			runtime.GC()
		}
	}()
}
