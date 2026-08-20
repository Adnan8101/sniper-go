package main
import (
	"flag"
	"fmt"
	"os"
	"time"
)
func main() {
	loopMode   := flag.Bool("loop", false, "Run in background loop mode continuously")
	cliMode    := flag.Bool("cli", false, "Run in interactive CLI mode")
	interval   := flag.Duration("interval", 4*time.Minute, "Refresh interval for loop mode (e.g. 4m, 30s)")
	configPath := flag.String("config", "config.json", "Path to config file")
	flag.Parse()
	fmt.Println("==========================================")
	fmt.Println(" Discord MFA Token Generator / Solver")
	fmt.Println("==========================================")
	if !loadConfigFile(*configPath) {
		os.Exit(1)
	}
	startConnectionKeepAlive()
	switch {
	case *cliMode:
		runMFAProcess()
		startInteractiveCLI()
	case *loopMode:
		fmt.Printf("[MFA] Running in loop mode (interval: %v)...\n", *interval)
		for {
			loadConfigFile(*configPath)
			if runMFAProcess() {
				fmt.Printf("[MFA] Success. Sleeping for %v...\n", *interval)
				time.Sleep(*interval)
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
