package main
import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)
type snipeResult struct {
	statusCode int
	body       string
	elapsedMs  float64
	err        error
}
func executeSnipe(vanity string) {
	if vanity == "" {
		fmt.Println("\x1b[33mUsage: snipe <vanity>\x1b[0m")
		return
	}
	if len(hotToken) == 0 {
		fmt.Println("\x1b[31m[SNIPE ERROR] Cannot snipe: token missing in config.json\x1b[0m")
		return
	}
	bodyBytes := buildBody(vanity)
	mfaToken  := []byte(cachedMfaToken)
	cookieHdr := []byte(getCookieHeader())
	salvoSize := config.SalvoSize
	if salvoSize <= 0 {
		salvoSize = 2
	}
	resChan := make(chan snipeResult, salvoSize)
	reqs    := make([]*fasthttp.Request, salvoSize)
	resps   := make([]*fasthttp.Response, salvoSize)
	for i := 0; i < salvoSize; i++ {
		req := fasthttp.AcquireRequest()
		req.SetRequestURIBytes(hotURL)
		req.Header.SetMethodBytes(hkMethod)
		req.Header.SetBytesKV(hkAuthorization,  hotToken)
		req.Header.SetBytesKV(hkUserAgent,       hotUserAgent)
		req.Header.SetBytesKV(hkXSuperProps,     hotSuperProp)
		req.Header.SetBytesKV(hkContentType,     hvContentType)
		req.Header.SetBytesKV(hkAcceptEncoding,  hvAcceptEncoding)
		req.Header.SetBytesKV(hkOrigin,          hotOrigin)
		req.Header.SetBytesKV(hkReferer,         hotReferer)
		if len(mfaToken) > 0 {
			req.Header.SetBytesKV(hkMFAAuth, mfaToken)
		}
		if len(cookieHdr) > 0 {
			req.Header.SetBytesKV(hkCookie, cookieHdr)
		}
		req.SetBody(bodyBytes)
		reqs[i]  = req
		resps[i] = fasthttp.AcquireResponse()
	}
	var wg sync.WaitGroup
	wg.Add(salvoSize)
	for i := 0; i < salvoSize; i++ {
		idx := i
		go func() {
			defer wg.Done()
			tStart := time.Now()
			err    := hostClient.Do(reqs[idx], resps[idx])
			elapsed := float64(time.Since(tStart).Microseconds()) / 1000.0
			resChan <- snipeResult{
				statusCode: resps[idx].StatusCode(),
				body:       string(resps[idx].Body()),
				elapsedMs:  elapsed,
				err:        err,
			}
		}()
	}
	go func() {
		wg.Wait()
		for i := 0; i < salvoSize; i++ {
			fasthttp.ReleaseRequest(reqs[i])
			fasthttp.ReleaseResponse(resps[i])
		}
		preWarmConns(salvoSize)
	}()
	res := <-resChan
	logSnipeResult(vanity, res)
}
func logSnipeResult(vanity string, res snipeResult) {
	fmt.Printf("\x1b[35m>> FIRE %s\x1b[0m\n", vanity)
	if res.err != nil {
		fmt.Printf("\x1b[31m[ ERR ] %s — %v | %.2fms\x1b[0m\n", vanity, res.err, res.elapsedMs)
		return
	}
	switch res.statusCode {
	case 200:
		fmt.Printf("\x1b[32m[ CLAIMED ] %s — HTTP 200 | %.2fms\x1b[0m\n", vanity, res.elapsedMs)
	case 400:
		switch {
		case containsCode(res.body, "50020"):
			fmt.Printf("\x1b[33m[ TAKEN ] %s — code already owned (50020) | %.2fms\x1b[0m\n", vanity, res.elapsedMs)
		case containsCode(res.body, "50024"):
			fmt.Printf("\x1b[33m[ INVALID ] %s — invalid code format (50024) | %.2fms\x1b[0m\n", vanity, res.elapsedMs)
		default:
			fmt.Printf("\x1b[31m[ FAIL 400 ] %s | %.2fms — %s\x1b[0m\n", vanity, res.elapsedMs, res.body)
		}
	case 401:
		fmt.Printf("\x1b[31m[ UNAUTH ] %s — token invalid or expired | %.2fms\x1b[0m\n", vanity, res.elapsedMs)
	case 403:
		fmt.Printf("\x1b[31m[ MFA NEEDED ] %s — run 'mfa' to refresh MFA token | %.2fms\x1b[0m\n", vanity, res.elapsedMs)
	case 429:
		fmt.Printf("\x1b[31m[ RATELIMIT ] %s — rate limited | %.2fms — retry_after: %s | %s\x1b[0m\n", vanity, res.elapsedMs, formatRetryAfter(res.body), res.body)
	default:
		fmt.Printf("\x1b[31m[ FAIL %d ] %s | %.2fms — %s\x1b[0m\n", res.statusCode, vanity, res.elapsedMs, res.body)
	}
}

func formatRetryAfter(body string) string {
	idx := strings.Index(body, `"retry_after":`)
	if idx == -1 {
		return "N/A"
	}
	start := idx + len(`"retry_after":`)
	end := start
	for end < len(body) && (body[end] >= '0' && body[end] <= '9' || body[end] == '.') {
		end++
	}
	if end > start {
		if sec, err := strconv.ParseFloat(body[start:end], 64); err == nil {
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
func containsCode(body, code string) bool {
	for i := 0; i <= len(body)-len(code); i++ {
		if body[i:i+len(code)] == code {
			return true
		}
	}
	return false
}
