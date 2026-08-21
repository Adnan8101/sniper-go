package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/valyala/fasthttp"
)

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

func setCommonHeaders(req *fasthttp.Request, tok, p string) {
	h := cfg.GetHost()
	req.Header.SetBytesK(kAuth, tok)
	req.Header.SetBytesKV(kUA, hotUA)
	req.Header.SetBytesKV(kProps, hotProp)
	req.Header.SetBytesKV(kCT, vCT)
	req.Header.SetBytesKV(kAE, vAE)
	req.Header.Set("Origin", "https://"+h)
	req.Header.Set("Referer", "https://"+h+p)
	if s := getCookieHeader(); s != "" {
		req.Header.Set("Cookie", s)
	}
}

func writeMFATokenToFile(tok string) {
	cachedMFA = tok
	hotMFA = []byte(tok)
	if err := os.WriteFile("mfa.txt", []byte(tok), 0644); err != nil {
		fmt.Println("[MFA ERROR] Failed to write MFA token to file:", err)
		return
	}
	ts := time.Now().Format("15:04:05")
	s := tok
	if len(tok) > 16 {
		s = tok[:8] + "..." + tok[len(tok)-8:]
	}
	fmt.Printf("\x1b[32m[MFA - %s] MFA solved via password & saved to mfa.txt: %s\x1b[0m\n", ts, s)
}

func sendMFA(tok, ticket, pass string) string {
	b, _ := json.Marshal(MFAPayload{Ticket: ticket, MFAType: "password", Data: pass})

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)

	h   := cfg.GetHost()
	ver := cfg.GetAPIVersion()
	req.SetRequestURI(fmt.Sprintf("https://%s/api/%s/mfa/finish", h, ver))
	req.Header.SetMethod("POST")
	setCommonHeaders(req, tok, "/login")
	req.SetBody(b)

	if err := hc.Do(req, res); err != nil {
		fmt.Printf("[MFA ERROR] Failed to send MFA solve request: %v\n", err)
		return "err"
	}

	absorbCookies(res)
	flushCookieCache()
	raw := res.Body()

	var r MFAResponse
	if err := json.Unmarshal(raw, &r); err == nil {
		if r.Token != "" {
			writeMFATokenToFile(r.Token)
			return r.Token
		}
		if r.Code != 0 || r.Message != "" {
			fmt.Printf("[MFA ERROR] MFA finish failed (code %d): %s\n", r.Code, r.Message)
			if r.Code == 60008 {
				fmt.Println("[MFA ERROR] Discord returned 60008 'Password does not match'. Check password in config.json.")
			}
			return "err"
		}
	}

	fmt.Printf("[MFA ERROR] HTTP Status %d: %s\n", res.StatusCode(), string(raw))
	return "err"
}

func runMFAProcess() bool {
	tok := cfg.GetToken()
	if tok == "" || cfg.Password == "" {
		fmt.Println("[MFA ERROR] Config invalid or missing. 'token' (or 'discordToken') and 'password' are required in config.json.")
		return false
	}

	h   := cfg.GetHost()
	ver := cfg.GetAPIVersion()
	gid := cfg.GuildID
	if gid == "" {
		gid = "1539670174221864963"
	}

	p := fmt.Sprintf("/api/%s/guilds/%s/vanity-url", ver, gid)
	fmt.Printf("[MFA] Requesting MFA ticket from https://%s%s...\n", h, p)

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(res)

	req.SetRequestURI(fmt.Sprintf("https://%s%s", h, p))
	req.Header.SetMethod("PATCH")
	setCommonHeaders(req, tok, fmt.Sprintf("/channels/%s", gid))
	req.SetBody([]byte(`{"code":"discord"}`))

	if err := hc.Do(req, res); err != nil {
		fmt.Printf("[MFA ERROR] Request failed: %v\n", err)
		return false
	}

	absorbCookies(res)
	flushCookieCache()
	raw := res.Body()
	st  := res.StatusCode()

	var v VanityResponse
	if err := json.Unmarshal(raw, &v); err != nil {
		fmt.Printf("[MFA ERROR] Invalid response JSON (status %d): %s\n", st, string(raw))
		return false
	}

	t := v.MFA.Ticket
	if t == "" {
		t = v.Ticket
	}

	if t == "" {
		switch {
		case v.Code == 40001 || st == 401:
			fmt.Printf("[MFA ERROR] Token is invalid or unauthorized (status %d): %s\n", st, string(raw))
		case strings.Contains(string(raw), "GUILD_INVALID_CODE"):
			fmt.Printf("[MFA INFO] No MFA challenge triggered (status %d GUILD_INVALID_CODE). An MFA session may already be active.\n", st)
		default:
			fmt.Printf("[MFA WARNING] No MFA ticket found in response (status %d): %s\n", st, string(raw))
		}
		return false
	}

	fmt.Printf("[MFA] Acquired MFA ticket: %s... Solving with password...\n", t[:10])
	newTok := sendMFA(tok, t, cfg.Password)
	return newTok != "" && newTok != "err"
}

func containsGuildInvalidCode(b []byte) bool {
	for i := 0; i <= len(b)-17; i++ {
		if b[i] == 'G' && string(b[i:i+17]) == "GUILD_INVALID_COD" {
			return true
		}
	}
	return false
}
