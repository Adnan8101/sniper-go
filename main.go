package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"time"
)

func main() {
	debug.SetGCPercent(400)

	loop   := flag.Bool("loop", false, "")
	cli    := flag.Bool("cli", false, "")
	target := flag.String("sniper", "", "")
	intv   := flag.Duration("interval", 4*time.Minute, "")
	cfgFile := flag.String("config", "config.json", "")
	flag.Parse()

	fmt.Println("==========================================")
	fmt.Println(" Discord MFA Token Generator / Solver")
	fmt.Println("==========================================")

	if !loadConfigFile(*cfgFile) {
		os.Exit(1)
	}

	startConnectionKeepAlive()

	switch {
	case *target != "":
		startContinuousSniper(*target)
	case *cli:
		runMFAProcess()
		startInteractiveCLI()
	case *loop:
		fmt.Printf("[MFA] Running in loop mode (interval: %v)...\n", *intv)
		for {
			loadConfigFile(*cfgFile)
			if runMFAProcess() {
				fmt.Printf("[MFA] Success. Sleeping for %v...\n", *intv)
				time.Sleep(*intv)
			} else {
				fmt.Println("[MFA] Attempt failed. Retrying in 10 seconds...")
				time.Sleep(10 * time.Second)
			}
		}
	default:
		if !runMFAProcess() {
			os.Exit(1)
		}
	}
}
