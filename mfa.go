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
func setCommonHeaders(req *fasthttp.Request, token string, path string) {
	host := config.GetHost()
	req.Header.Set("Authorization", token)
	req.Header.Set("User-Agent", config.UserAgent())
	req.Header.Set("X-Super-Properties", config.BuildSuperProperties())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Origin", "https://"+host)
	req.Header.Set("Referer", "https://"+host+path)
	if ch := getCookieHeader(); ch != "" {
		req.Header.Set("Cookie", ch)
	}
}
func writeMFATokenToFile(token string) {
	cachedMfaToken = token
	if err := os.WriteFile("mfa.txt", []byte(token), 0644); err != nil {
		fmt.Println("[MFA ERROR] Failed to write MFA token to file:", err)
		return
	}
	now := time.Now().Format("15:04:05")
	display := token
	if len(token) > 16 {
		display = token[:8] + "..." + token[len(token)-8:]
	}
	fmt.Printf("\x1b[32m[MFA - %s] MFA solved via password & saved to mfa.txt: %s\x1b[0m\n", now, display)
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
	if err := hostClient.Do(req, resp); err != nil {
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
	if err := hostClient.Do(req, resp); err != nil {
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
		switch {
		case v.Code == 40001 || statusCode == 401:
			fmt.Printf("[MFA ERROR] Token is invalid or unauthorized (status %d): %s\n", statusCode, string(respBody))
		case strings.Contains(string(respBody), "GUILD_INVALID_CODE"):
			fmt.Printf("[MFA INFO] No MFA challenge triggered (status %d GUILD_INVALID_CODE). An MFA session may already be active.\n", statusCode)
		default:
			fmt.Printf("[MFA WARNING] No MFA ticket found in response (status %d): %s\n", statusCode, string(respBody))
		}
		return false
	}
	fmt.Printf("[MFA] Acquired MFA ticket: %s... Solving with password...\n", ticket[:10])
	newToken := sendMFA(token, ticket, config.Password)
	return newToken != "" && newToken != "err"
}
func containsGuildInvalidCode(body []byte) bool {
	for i := 0; i <= len(body)-17; i++ {
		if body[i] == 'G' && string(body[i:i+17]) == "GUILD_INVALID_COD" {
			return true
		}
	}
	return false
}
