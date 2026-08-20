package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
	loop := flag.Bool("loop", false, "Run in background loop mode continuously")
	cli := flag.Bool("cli", false, "Run in interactive CLI mode")
	target := flag.String("sniper", "", "Start continuous high-speed vanity sniper & locker for target vanity")
	intv := flag.Duration("interval", 4*time.Minute, "Refresh interval for loop mode (e.g. 4m, 30s)")
	cfgFile := flag.String("config", "config.json", "Path to config file")
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
