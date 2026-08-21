package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func printCLIHelp() {
	fmt.Print("\nCommands:\n" +
		"  sniper <vanity>  start continuous high-speed vanity sniper & locker\n" +
		"  snipe <vanity>   fire a single manual vanity claim salvo\n" +
		"  mfa | refresh    force an immediate MFA token solve & refresh\n" +
		"  status           show current MFA token & engine status\n" +
		"  config           show active configuration parameters\n" +
		"  help             show this help menu\n" +
		"  exit | quit      exit the program\n\n")
}

func printCLIStatus() {
	fmt.Println("\n=== MFA Engine Status ===")
	tok := cfg.GetToken()
	hasTok := tok != ""
	hasPass := cfg.Password != ""
	snip := "none"
	if b, err := os.ReadFile("mfa.txt"); err == nil && len(b) > 16 {
		s := string(b)
		snip = s[:8] + "..." + s[len(s)-8:]
	}
	fmt.Printf("  Discord Host : %s\n", cfg.GetHost())
	fmt.Printf("  API Version  : %s\n", cfg.GetAPIVersion())
	fmt.Printf("  Guild ID     : %s\n", cfg.GuildID)
	fmt.Printf("  User Token   : %t\n", hasTok)
	fmt.Printf("  Password     : %t\n", hasPass)
	fmt.Printf("  Saved MFA    : %s\n\n", snip)
}

func startInteractiveCLI() {
	r := bufio.NewReader(os.Stdin)
	fmt.Println("\nInteractive CLI Ready. Type 'sniper <vanity>', 'snipe <vanity>', 'mfa', or 'help' for options.")
	for {
		fmt.Print("sniper> ")
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}
		args := strings.Fields(strings.TrimSpace(line))
		if len(args) == 0 {
			continue
		}
		cmd := strings.ToLower(args[0])
		switch cmd {
		case "sniper", "locker":
			v := ""
			if len(args) > 1 {
				v = args[1]
			}
			if loadConfigFile("config.json") {
				startContinuousSniper(v)
			}
		case "snipe":
			v := ""
			if len(args) > 1 {
				v = args[1]
			}
			if loadConfigFile("config.json") {
				executeSnipe(v)
			}
		case "mfa", "refresh":
			if loadConfigFile("config.json") {
				runMFAProcess()
			}
		case "status":
			loadConfigFile("config.json")
			printCLIStatus()
		case "config":
			if loadConfigFile("config.json") {
				fmt.Printf("\nHost: %s, API: %s, GuildID: %s, OS: %s, Browser: %s\n\n",
					cfg.GetHost(), cfg.GetAPIVersion(), cfg.GuildID, cfg.OS, cfg.Browser)
			}
		case "help":
			printCLIHelp()
		case "exit", "quit":
			fmt.Println("Exiting CLI.")
			return
		default:
			fmt.Printf("Unknown command: %s (type 'help' for command list)\n", cmd)
		}
	}
}
