package main

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)


var (
	config         Config
	cookieMu       sync.Mutex
	cookieStore    = make(map[string]string)
	fastHttpClient = &fasthttp.Client{
		TLSConfig: &tls.Config{
			InsecureSkipVerify:       true,
			PreferServerCipherSuites: true,
			SessionTicketsDisabled:   false,
			ClientSessionCache:       tls.NewLRUClientSessionCache(200),
		},
		MaxConnsPerHost:               500,
		MaxIdleConnDuration:           600 * time.Second,
		ReadTimeout:                   3 * time.Second,
		WriteTimeout:                  3 * time.Second,
		MaxResponseBodySize:           512 * 1024,
		DisableHeaderNamesNormalizing: true,
	}
)

func warmUpConnection() {
	host := config.GetHost()
	for i := 0; i < 4; i++ {
		go func() {
			req := fasthttp.AcquireRequest()
			defer fasthttp.ReleaseRequest(req)
			resp := fasthttp.AcquireResponse()
			defer fasthttp.ReleaseResponse(resp)

			req.SetRequestURI(fmt.Sprintf("https://%s/api/v9/gateway", host))
			req.Header.SetMethod("GET")
			req.Header.Set("User-Agent", config.UserAgent())
			_ = fastHttpClient.Do(req, resp)
		}()
	}
}

func startConnectionKeepAlive() {
	warmUpConnection()
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		for range ticker.C {
			warmUpConnection()
		}
	}()
}



type Config struct {
	Token        string `json:"token"`
	DiscordToken string `json:"discordToken"`
	Password     string `json:"password"`
	GuildID      string `json:"guildId"`
	DiscordHost  string `json:"discordHost"`
	APIVersion   string `json:"apiVersion"`
	OS           string `json:"os"`
	Browser      string `json:"browser"`
	Device       string `json:"device"`
}

func (c *Config) GetToken() string {
	if c.Token != "" {
		return c.Token
	}
	return c.DiscordToken
}

func (c *Config) GetHost() string {
	if c.DiscordHost != "" {
		return c.DiscordHost
	}
	return "canary.discord.com"
}

func (c *Config) GetAPIVersion() string {
	if c.APIVersion != "" {
		return c.APIVersion
	}
	return "v9"
}

func (c *Config) UserAgent() string {
	return "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0"
}

func (c *Config) BuildSuperProperties() string {
	releaseChannel := "stable"
	if strings.HasPrefix(c.GetHost(), "canary") {
		releaseChannel = "canary"
	}
	osName := c.OS
	if osName == "" {
		osName = "Windows"
	}
	browserName := c.Browser
	if browserName == "" {
		browserName = "Firefox"
	}

	props := map[string]interface{}{
		"os":                       osName,
		"browser":                  browserName,
		"device":                   c.Device,
		"system_locale":            "en-US",
		"browser_user_agent":       c.UserAgent(),
		"browser_version":          "133.0",
		"os_version":               "10",
		"referrer":                 "https://www.google.com/",
		"referring_domain":         "www.google.com",
		"search_engine":            "google",
		"referrer_current":         "",
		"referring_domain_current": "",
		"release_channel":          releaseChannel,
		"client_build_number":      356140,
		"client_event_source":      nil,
		"has_client_mods":          false,
	}

	data, _ := json.Marshal(props)
	return base64.StdEncoding.EncodeToString(data)
}

func absorbCookies(resp *fasthttp.Response) {
	cookieMu.Lock()
	defer cookieMu.Unlock()
	resp.Header.VisitAllCookie(func(key, value []byte) {
		c := fasthttp.AcquireCookie()
		defer fasthttp.ReleaseCookie(c)
		c.ParseBytes(value)
		k := string(c.Key())
		v := string(c.Value())
		if k != "" {
			cookieStore[k] = v
		}
	})
}

func getCookieHeader() string {
	cookieMu.Lock()
	defer cookieMu.Unlock()
	if len(cookieStore) == 0 {
		return ""
	}
	var pairs []string
	for k, v := range cookieStore {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	return strings.Join(pairs, "; ")
}

type MFAPayload struct {
	Ticket  string `json:"ticket"`
	MFAType string `json:"mfa_type"`
	Data    string `json:"data"`
}

type MFAResponse struct {
	Token   string `json:"token"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type VanityResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	MFA     struct {
		Ticket  string                   `json:"ticket"`
		Methods []map[string]interface{} `json:"methods"`
	} `json:"mfa"`
	Ticket string `json:"ticket"`
}

func setCommonHeaders(req *fasthttp.Request, token string, path string) {
	host := config.GetHost()
	req.Header.Set("Authorization", token)
	req.Header.Set("User-Agent", config.UserAgent())
	req.Header.Set("X-Super-Properties", config.BuildSuperProperties())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Origin", "https://"+host)
	req.Header.Set("Referer", "https://"+host+path)

	cookieHeader := getCookieHeader()
	if cookieHeader != "" {
		req.Header.Set("Cookie", cookieHeader)
	}
}

func writeMFATokenToFile(token string) {
	if err := os.WriteFile("mfa.txt", []byte(token), 0644); err != nil {
		fmt.Println("[MFA ERROR] Failed to write MFA token to file:", err)
	} else {
		now := time.Now().Format("15:04:05")
		tokenDisplay := token
		if len(token) > 16 {
			tokenDisplay = token[:8] + "..." + token[len(token)-8:]
		}
		fmt.Printf("\x1b[32m[MFA - %s] MFA solved via password & saved to mfa.txt: %s\x1b[0m\n", now, tokenDisplay)
	}
}

func sendMFA(token, ticket, password string) string {
	payload := MFAPayload{Ticket: ticket, MFAType: "password", Data: password}
	jsonPayload, _ := json.Marshal(payload)

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	host := config.GetHost()
	apiVer := config.GetAPIVersion()
	req.SetRequestURI(fmt.Sprintf("https://%s/api/%s/mfa/finish", host, apiVer))
	req.Header.SetMethod("POST")
	setCommonHeaders(req, token, "/login")
	req.SetBody(jsonPayload)

	if err := fastHttpClient.Do(req, resp); err != nil {
		fmt.Printf("[MFA ERROR] Failed to send MFA solve request: %v\n", err)
		return "err"
	}

	absorbCookies(resp)

	body := resp.Body()
	var mfaResp MFAResponse
	if err := json.Unmarshal(body, &mfaResp); err == nil {
		if mfaResp.Token != "" {
			writeMFATokenToFile(mfaResp.Token)
			return mfaResp.Token
		}
		if mfaResp.Code != 0 || mfaResp.Message != "" {
			fmt.Printf("[MFA ERROR] MFA finish failed (code %d): %s\n", mfaResp.Code, mfaResp.Message)
			if mfaResp.Code == 60008 {
				fmt.Println("[MFA ERROR] Discord returned 60008 'Password does not match'. Check password in config.json.")
			}
			return "err"
		}
	}

	fmt.Printf("[MFA ERROR] HTTP Status %d: %s\n", resp.StatusCode(), string(body))
	return "err"
}

func runMFAProcess() bool {
	token := config.GetToken()
	if token == "" || config.Password == "" {
		fmt.Println("[MFA ERROR] Config invalid or missing. 'token' (or 'discordToken') and 'password' are required in config.json.")
		return false
	}

	host := config.GetHost()
	apiVer := config.GetAPIVersion()
	guildID := config.GuildID
	if guildID == "" {
		guildID = "1539670174221864963"
	}

	vanityPath := fmt.Sprintf("/api/%s/guilds/%s/vanity-url", apiVer, guildID)
	fmt.Printf("[MFA] Requesting MFA ticket from https://%s%s...\n", host, vanityPath)

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(fmt.Sprintf("https://%s%s", host, vanityPath))
	req.Header.SetMethod("PATCH")
	setCommonHeaders(req, token, fmt.Sprintf("/channels/%s", guildID))
	req.SetBody([]byte(`{"code":"discord"}`))

	if err := fastHttpClient.Do(req, resp); err != nil {

		fmt.Printf("[MFA ERROR] Request failed: %v\n", err)
		return false
	}

	absorbCookies(resp)

	respBody := resp.Body()
	statusCode := resp.StatusCode()

	var v VanityResponse
	if err := json.Unmarshal(respBody, &v); err != nil {
		fmt.Printf("[MFA ERROR] Invalid response JSON (status %d): %s\n", statusCode, string(respBody))
		return false
	}

	ticket := v.MFA.Ticket
	if ticket == "" {
		ticket = v.Ticket
	}

	if ticket == "" {
		if v.Code == 40001 || statusCode == 401 {
			fmt.Printf("[MFA ERROR] Token is invalid or unauthorized (status %d): %s\n", statusCode, string(respBody))
		} else {
			fmt.Printf("[MFA WARNING] No MFA ticket found in response (status %d): %s\n", statusCode, string(respBody))
		}
		return false
	}

	fmt.Printf("[MFA] Acquired MFA ticket: %s... Solving with password...\n", ticket[:10])
	newToken := sendMFA(token, ticket, config.Password)
	return newToken != "" && newToken != "err"
}

func loadConfigFile(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("[MFA ERROR] Failed to read %s: %v\n", path, err)
		return false
	}
	if err := json.Unmarshal(data, &config); err != nil {
		fmt.Printf("[MFA ERROR] Failed to parse %s: %v\n", path, err)
		return false
	}
	return true
}



func executeSnipe(vanity string) {
	if vanity == "" {
		fmt.Println("\x1b[33mUsage: snipe <vanity>\x1b[0m")
		return
	}

	token := config.GetToken()
	if token == "" {
		fmt.Println("\x1b[31m[MFA ERROR] Cannot snipe: token missing in config.json\x1b[0m")
		return
	}

	host := config.GetHost()
	apiVer := config.GetAPIVersion()
	guildID := config.GuildID
	if guildID == "" {
		guildID = "1539670174221864963"
	}

	vanityPath := fmt.Sprintf("/api/%s/guilds/%s/vanity-url", apiVer, guildID)
	reqURL := fmt.Sprintf("https://%s%s", host, vanityPath)
	body := []byte(fmt.Sprintf(`{"code":"%s"}`, vanity))
	mfaTokenStr := ""
	if mfaData, err := os.ReadFile("mfa.txt"); err == nil && len(mfaData) > 0 {
		mfaTokenStr = strings.TrimSpace(string(mfaData))
	}

	fmt.Printf("\x1b[35m>> FIRE %s\x1b[0m\n", vanity)
	start := time.Now()

	type snipeResult struct {
		statusCode int
		body       string
		elapsed    int64
		err        error
	}

	salvoSize := 2
	resChan := make(chan snipeResult, salvoSize)

	for i := 0; i < salvoSize; i++ {
		go func() {
			req := fasthttp.AcquireRequest()
			defer fasthttp.ReleaseRequest(req)
			resp := fasthttp.AcquireResponse()
			defer fasthttp.ReleaseResponse(resp)

			req.SetRequestURI(reqURL)
			req.Header.SetMethod("PATCH")
			setCommonHeaders(req, token, fmt.Sprintf("/channels/%s", guildID))
			if mfaTokenStr != "" {
				req.Header.Set("X-Discord-MFA-Authorization", mfaTokenStr)
			}
			req.SetBody(body)

			t0 := time.Now()
			err := fastHttpClient.Do(req, resp)
			elapsed := time.Since(t0).Milliseconds()

			if err != nil {
				resChan <- snipeResult{err: err, elapsed: elapsed}
				return
			}
			absorbCookies(resp)
			resChan <- snipeResult{
				statusCode: resp.StatusCode(),
				body:       string(resp.Body()),
				elapsed:    elapsed,
			}
		}()
	}

	res := <-resChan
	totalElapsed := time.Since(start).Milliseconds()

	if res.err != nil {
		fmt.Printf("\x1b[31mFAILED %s\x1b[0m  %v  roundTrip=%dms\n", vanity, res.err, totalElapsed)
		return
	}

	if res.statusCode == 200 {
		fmt.Printf("\x1b[32m[ ms ] HTTP 200 --> %d ms | CLAIMED %s\x1b[0m\n", res.elapsed, vanity)
	} else {
		fmt.Printf("\x1b[31mFAILED %s\x1b[0m  %s  roundTrip=%dms  statusCode=%d\n", vanity, res.body, res.elapsed, res.statusCode)
	}
}



func printCLIHelp() {
	fmt.Println("\nCommands:")
	fmt.Println("  mfa | refresh    force an immediate MFA token solve & refresh")
	fmt.Println("  snipe <vanity>   fire a manual vanity claim right now")
	fmt.Println("  status           show current MFA token & engine status")
	fmt.Println("  config           show active configuration parameters")
	fmt.Println("  help             show this help menu")
	fmt.Println("  exit | quit      exit the program\n")
}

func printCLIStatus() {
	fmt.Println("\n=== MFA Engine Status ===")
	token := config.GetToken()
	hasToken := token != ""
	hasPass := config.Password != ""
	tokenSnippet := "none"
	if data, err := os.ReadFile("mfa.txt"); err == nil && len(data) > 16 {
		tokenStr := string(data)
		tokenSnippet = tokenStr[:8] + "..." + tokenStr[len(tokenStr)-8:]
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


func main() {
	loopMode := flag.Bool("loop", false, "Run in background loop mode continuously")
	cliMode := flag.Bool("cli", false, "Run in interactive CLI mode")
	interval := flag.Duration("interval", 4*time.Minute, "Refresh interval for loop mode (e.g. 4m, 30s)")
	configPath := flag.String("config", "config.json", "Path to config file")
	flag.Parse()

	fmt.Println("==========================================")
	fmt.Println(" Discord MFA Token Generator / Solver")
	fmt.Println("==========================================")

	if !loadConfigFile(*configPath) {
		os.Exit(1)
	}

	startConnectionKeepAlive()

	if *cliMode {

		runMFAProcess()
		startInteractiveCLI()
		return
	}

	if *loopMode {
		fmt.Printf("[MFA] Running in loop mode (interval: %v)...\n", *interval)
		for {
			loadConfigFile(*configPath)
			success := runMFAProcess()
			if success {
				fmt.Printf("[MFA] Success. Sleeping for %v...\n", *interval)
				time.Sleep(*interval)
			} else {
				fmt.Println("[MFA] Attempt failed. Retrying in 10 seconds...")
				time.Sleep(10 * time.Second)
			}
		}
	}

	// Default: single-shot mode (solve once & exit immediately)
	success := runMFAProcess()
	if !success {
		os.Exit(1)
	}
}