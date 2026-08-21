package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
)

type snipeResult struct {
	statusCode int
	body       string
	elapsedMs  float64
	err        error
}

type GuildInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Features    []string `json:"features"`
	Permissions string   `json:"permissions"`
	PremiumTier int      `json:"premium_tier"`
}

// UserGuild is the partial guild object returned by /users/@me/guilds, whose
// Permissions field is the *caller's* actual permission bitfield for that
// guild — unlike GET /guilds/{id}, which doesn't return one for this token.
type UserGuild struct {
	ID          string `json:"id"`
	Permissions string `json:"permissions"`
}

func hasAdminInGuild(gid string) (found, isAdmin bool) {
	h := cfg.GetHost()
	ver := cfg.GetAPIVersion()
	url := fmt.Sprintf("https://%s/api/%s/users/@me/guilds", h, ver)

	st, body, err := doAPIRequest("GET", url, "/channels/@me", cfg.GetToken(), nil)
	if err != nil || st != 200 {
		return false, false
	}

	var guilds []UserGuild
	if err := json.Unmarshal(body, &guilds); err != nil {
		return false, false
	}

	for _, g := range guilds {
		if g.ID != gid {
			continue
		}
		if g.Permissions == "" {
			return true, false
		}
		val, err := strconv.ParseUint(g.Permissions, 10, 64)
		if err != nil {
			return true, false
		}
		return true, (val&0x8 != 0) || (val&0x20 != 0)
	}
	return false, false
}

func verifyGuildAndPermissions() bool {
	gid := cfg.GuildID
	if gid == "" {
		gid = "1539670174221864963"
	}
	h := cfg.GetHost()
	ver := cfg.GetAPIVersion()
	u := fmt.Sprintf("https://%s/api/%s/guilds/%s", h, ver, gid)

	st, b, err := doAPIRequest("GET", u, fmt.Sprintf("/channels/%s", gid), cfg.GetToken(), nil)
	if err != nil {
		fmt.Printf("\x1b[31m[VERIFY ERROR] Failed to fetch guild details: %v\x1b[0m\n", err)
		return false
	}
	if st != 200 {
		fmt.Printf("\x1b[31m[VERIFY ERROR] Guild API returned HTTP %d: %s\x1b[0m\n", st, string(b))
		return false
	}

	var g GuildInfo
	if err := json.Unmarshal(b, &g); err != nil {
		fmt.Printf("\x1b[31m[VERIFY ERROR] Failed to parse guild JSON: %v\x1b[0m\n", err)
		return false
	}

	found, hasAdmin := hasAdminInGuild(gid)

	hasVanity := false
	for _, f := range g.Features {
		if f == "VANITY_URL" {
			hasVanity = true
			break
		}
	}

	fmt.Printf("\x1b[36m[VERIFY] Guild: %s (%s)\x1b[0m\n", g.Name, g.ID)
	switch {
	case !found:
		fmt.Println("\x1b[33m[VERIFY WARNING] Could not confirm permissions — guild not found in /users/@me/guilds.\x1b[0m")
	case !hasAdmin:
		fmt.Println("\x1b[33m[VERIFY WARNING] Account permission check: Administrator/Manage Guild bit missing!\x1b[0m")
	default:
		fmt.Println("\x1b[32m[VERIFY PASS] Admin permissions confirmed.\x1b[0m")
	}

	if !hasVanity && g.PremiumTier < 3 {
		fmt.Printf("\x1b[33m[VERIFY WARNING] Guild feature 'VANITY_URL' not active (Boost Tier: %d). Vanity update might require boosts.\x1b[0m\n", g.PremiumTier)
	} else {
		fmt.Println("\x1b[32m[VERIFY PASS] Server has Vanity URL / Boost unlock enabled.\x1b[0m")
	}

	return true
}

func fireOne(req *fasthttp.Request, res *fasthttp.Response) snipeResult {
	atomic.AddInt64(&fireCount, 1)
	t0 := time.Now()
	err := hc.Do(req, res)
	ms := float64(time.Since(t0).Microseconds()) / 1000.0
	return snipeResult{
		statusCode: res.StatusCode(),
		body:       string(res.Body()),
		elapsedMs:  ms,
		err:        err,
	}
}

// pickSalvoResult picks the winner out of a salvo: any response that actually
// claimed or already owns the vanity wins, even if it wasn't the first to
// finish. Only if none succeeded do we fall back to the fastest result, since
// that's the most representative one for logging/backoff decisions.
func pickSalvoResult(results []snipeResult) snipeResult {
	for _, r := range results {
		if r.err != nil {
			continue
		}
		if r.statusCode == 200 || (r.statusCode == 400 && strings.Contains(r.body, "50020")) {
			return r
		}
	}
	best := results[0]
	for _, r := range results[1:] {
		if r.elapsedMs < best.elapsedMs {
			best = r
		}
	}
	return best
}

func executeSnipeSalvo(code string) snipeResult {
	body := buildBody(code)
	mfa := currentMFA()
	cookie := currentCookie()
	n := cfg.SalvoSize
	if n <= 0 {
		n = 2
	}

	reqs := make([]*fasthttp.Request, n)
	resps := make([]*fasthttp.Response, n)
	for i := 0; i < n; i++ {
		req := fasthttp.AcquireRequest()
		req.SetRequestURIBytes(hotURL)
		req.Header.SetMethodBytes(kMethod)
		req.Header.SetBytesKV(kAuth, hotTok)
		req.Header.SetBytesKV(kUA, hotUA)
		req.Header.SetBytesKV(kProps, hotProp)
		req.Header.SetBytesKV(kCT, vCT)
		req.Header.SetBytesKV(kAE, vAE)
		req.Header.SetBytesKV(kOrigin, hotOrigin)
		req.Header.SetBytesKV(kRef, hotRef)
		if len(mfa) > 0 {
			req.Header.SetBytesKV(kMFA, mfa)
		}
		if len(cookie) > 0 {
			req.Header.SetBytesKV(kCookie, cookie)
		}
		req.SetBody(body)
		reqs[i] = req
		resps[i] = fasthttp.AcquireResponse()
	}

	results := make([]snipeResult, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		idx := i
		go func() {
			defer wg.Done()
			results[idx] = fireOne(reqs[idx], resps[idx])
		}()
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		fasthttp.ReleaseRequest(reqs[i])
		fasthttp.ReleaseResponse(resps[i])
	}

	// fasthttp already returns these connections to HostClient's own idle
	// pool once each Do() completes — no need to force-dial replacements
	// here, that would just double the handshake work on every shot.
	return pickSalvoResult(results)
}

func executeSnipe(code string) {
	if code == "" {
		fmt.Println("\x1b[33mUsage: snipe <vanity>\x1b[0m")
		return
	}
	if len(hotTok) == 0 {
		fmt.Println("\x1b[31m[SNIPE ERROR] Cannot snipe: token missing in config.json\x1b[0m")
		return
	}
	res := executeSnipeSalvo(code)
	logSnipeResult(code, res)
}

func startContinuousSniper(code string) {
	if code == "" {
		fmt.Println("\x1b[33mUsage: sniper <vanity> (or locker <vanity>)\x1b[0m")
		return
	}

	fmt.Println("\n\x1b[36m==============================================")
	fmt.Printf(" Starting Continuous Vanity Sniper for '%s'\n", code)
	fmt.Println("==============================================\x1b[0m")

	fmt.Println("\x1b[34m[STEP 1/3] Checking MFA session...\x1b[0m")
	if cachedMFA == "" {
		if !runMFAProcess() {
			fmt.Println("\x1b[31m[SNIPER ERROR] MFA solve failed. Cannot proceed.\x1b[0m")
			return
		}
	} else {
		fmt.Println("\x1b[32m[MFA OK] Active MFA token found.\x1b[0m")
	}

	fmt.Println("\x1b[34m[STEP 2/3] Verifying Guild Admin Permissions & Vanity status...\x1b[0m")
	if !verifyGuildAndPermissions() {
		fmt.Println("\x1b[31m[SNIPER ERROR] Guild verification failed. Check guildId in config.json.\x1b[0m")
		return
	}

	fmt.Println("\x1b[34m[STEP 3/3] Launching Continuous Snipe Engine...\x1b[0m")
	fmt.Printf("\x1b[32m[SNIPER READY] Monitoring and claiming '%s' at max speed. Press Ctrl+C or type exit to stop.\x1b[0m\n\n", code)

	cd := time.Duration(cfg.SnipeCooldownMs) * time.Millisecond
	if cd <= 0 {
		cd = 50 * time.Millisecond
	}

	for {
		res := executeSnipeSalvo(code)
		logSnipeResult(code, res)

		if res.statusCode == 200 {
			fmt.Printf("\n\x1b[32m🎉 SUCCESS! Vanity '%s' successfully claimed for your server!\x1b[0m\n", code)
			break
		}
		if res.statusCode == 400 && strings.Contains(res.body, "50020") {
			fmt.Printf("\n\x1b[32m✅ Vanity '%s' is already owned by this server!\x1b[0m\n", code)
			break
		}
		if res.statusCode == 403 {
			fmt.Println("\x1b[31m[MFA EXPIRED] Refreshing MFA session...\x1b[0m")
			runMFAProcess()
		}

		if res.statusCode == 429 {
			sec := parseRetryAfterSec(res.body)
			if sec > 0 {
				dur := time.Duration(sec*1000) * time.Millisecond
				fmt.Printf("\x1b[33m[RL BACKOFF] Sleeping for %.2fs to avoid IP ban...\x1b[0m\n", sec)
				time.Sleep(dur)
				continue
			}
		}

		time.Sleep(cd)
	}
}

func logSnipeResult(code string, res snipeResult) {
	fmt.Printf("\x1b[35m>> FIRE %s\x1b[0m\n", code)
	if res.err != nil {
		fmt.Printf("\x1b[31m[ ERR ] %s — %v | %.2fms\x1b[0m\n", code, res.err, res.elapsedMs)
		return
	}
	switch res.statusCode {
	case 200:
		fmt.Printf("\x1b[32m[ CLAIMED ] %s — HTTP 200 | %.2fms\x1b[0m\n", code, res.elapsedMs)
	case 400:
		switch {
		case strings.Contains(res.body, "50020"):
			fmt.Printf("\x1b[33m[ TAKEN ] %s — code already owned (50020) | %.2fms\x1b[0m\n", code, res.elapsedMs)
		case strings.Contains(res.body, "50024"):
			fmt.Printf("\x1b[33m[ INVALID ] %s — invalid code format (50024) | %.2fms\x1b[0m\n", code, res.elapsedMs)
		default:
			fmt.Printf("\x1b[31m[ FAIL 400 ] %s | %.2fms — %s\x1b[0m\n", code, res.elapsedMs, res.body)
		}
	case 401:
		fmt.Printf("\x1b[31m[ UNAUTH ] %s — token invalid or expired | %.2fms\x1b[0m\n", code, res.elapsedMs)
	case 403:
		fmt.Printf("\x1b[31m[ MFA NEEDED ] %s — run 'mfa' to refresh MFA token | %.2fms\x1b[0m\n", code, res.elapsedMs)
	case 429:
		fmt.Printf("\x1b[31m[ RATELIMIT ] %s — rate limited | %.2fms — retry_after: %s | %s\x1b[0m\n", code, res.elapsedMs, formatRetryAfter(res.body), res.body)
	default:
		fmt.Printf("\x1b[31m[ FAIL %d ] %s | %.2fms — %s\x1b[0m\n", res.statusCode, code, res.elapsedMs, res.body)
	}
}

func parseRetryAfterSec(s string) float64 {
	i := strings.Index(s, `"retry_after":`)
	if i == -1 {
		return 0
	}
	start := i + len(`"retry_after":`)
	end := start
	for end < len(s) && (s[end] >= '0' && s[end] <= '9' || s[end] == '.') {
		end++
	}
	if end > start {
		if sec, err := strconv.ParseFloat(s[start:end], 64); err == nil {
			return sec
		}
	}
	return 0
}

func formatRetryAfter(s string) string {
	i := strings.Index(s, `"retry_after":`)
	if i == -1 {
		return "N/A"
	}
	start := i + len(`"retry_after":`)
	end := start
	for end < len(s) && (s[end] >= '0' && s[end] <= '9' || s[end] == '.') {
		end++
	}
	if end > start {
		if sec, err := strconv.ParseFloat(s[start:end], 64); err == nil {
			hrs := sec / 3600.0
			mins := sec / 60.0
			if hrs >= 1.0 {
				return fmt.Sprintf("%.2f hours (%.0fs)", hrs, sec)
			}
			if mins >= 1.0 {
				return fmt.Sprintf("%.2f mins (%.0fs)", mins, sec)
			}
			return fmt.Sprintf("%.2fs", sec)
		}
	}
	return "N/A"
}
