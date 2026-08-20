package main
import (
	"bufio"
	"fmt"
	"os"
	"strings"
)
func printCLIHelp() {
	fmt.Print("\nCommands:\n" +
		"  mfa | refresh    force an immediate MFA token solve & refresh\n" +
		"  snipe <vanity>   fire a manual vanity claim right now\n" +
		"  status           show current MFA token & engine status\n" +
		"  config           show active configuration parameters\n" +
		"  help             show this help menu\n" +
		"  exit | quit      exit the program\n\n")
}
func printCLIStatus() {
	fmt.Println("\n=== MFA Engine Status ===")
	token := config.GetToken()
	hasToken := token != ""
	hasPass := config.Password != ""
	tokenSnippet := "none"
	if data, err := os.ReadFile("mfa.txt"); err == nil && len(data) > 16 {
		s := string(data)
		tokenSnippet = s[:8] + "..." + s[len(s)-8:]
	}
	fmt.Printf("  Discord Host : %s\n", config.GetHost())
	fmt.Printf("  API Version  : %s\n", config.GetAPIVersion())
	fmt.Printf("  Guild ID     : %s\n", config.GuildID)
	fmt.Printf("  User Token   : %t\n", hasToken)
	fmt.Printf("  Password     : %t\n", hasPass)
	fmt.Printf("  Saved MFA    : %s\n\n", tokenSnippet)
}
func startInteractiveCLI() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("\nInteractive CLI Ready. Type 'snipe <vanity>', 'mfa', or 'help' for options.")
	for {
		fmt.Print("sniper> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		parts := strings.Fields(strings.TrimSpace(line))
		if len(parts) == 0 {
			continue
		}
		cmd := strings.ToLower(parts[0])
		switch cmd {
		case "snipe":
			vanity := ""
			if len(parts) > 1 {
				vanity = parts[1]
			}
			loadConfigFile("config.json")
			executeSnipe(vanity)
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
					config.GetHost(), config.GetAPIVersion(), config.GuildID, config.OS, config.Browser)
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
