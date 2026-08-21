package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
)

// maxIdleConnDuration bounds how long fasthttp keeps an idle connection alive
// before its connsCleaner closes it. Left at the zero value this defaults to
// fasthttp.DefaultMaxIdleConnDuration (10s) — shorter than any reasonable gap
// between snipe attempts, which silently emptied the warm pool. Set explicitly
// so the keep-alive ticker in client.go has a real window to refill against.
const maxIdleConnDuration = 120 * time.Second

var (
	hotURL, hotRef, hotOrigin, hotTok, hotProp, hotUA []byte
	hotAddr                                           string
	warmURL                                           []byte
)

var (
	kMethod = []byte("PATCH")
	kAuth   = []byte("Authorization")
	kUA     = []byte("User-Agent")
	kProps  = []byte("X-Super-Properties")
	kCT     = []byte("Content-Type")
	kAE     = []byte("Accept-Encoding")
	kOrigin = []byte("Origin")
	kRef    = []byte("Referer")
	kMFA    = []byte("X-Discord-MFA-Authorization")
	kCookie = []byte("Cookie")
	vCT     = []byte("application/json")
	vAE     = []byte("identity")
)

func buildBody(code string) []byte {
	prefix := `{"code":"`
	suffix := `"}`
	buf := make([]byte, 0, len(prefix)+len(code)+len(suffix))
	buf = append(buf, prefix...)
	buf = append(buf, code...)
	buf = append(buf, suffix...)
	return buf
}

func rebuildHotCache() {
	h   := cfg.GetHost()
	ver := cfg.GetAPIVersion()
	gid := cfg.GuildID
	if gid == "" {
		gid = "1539670174221864963"
	}

	hotURL    = []byte(fmt.Sprintf("https://%s/api/%s/guilds/%s/vanity-url", h, ver, gid))
	hotRef    = []byte(fmt.Sprintf("https://%s/channels/%s", h, gid))
	hotOrigin = []byte("https://" + h)
	hotTok    = []byte(cfg.GetToken())
	hotProp   = []byte(cfg.BuildSuperProperties())
	hotUA     = []byte(cfg.UserAgent())
	warmURL   = []byte(fmt.Sprintf("https://%s/api/v9/gateway", h))

	initHostClient(h)
}

func initHostClient(h string) {
	addr := h + ":443"
	if hc != nil && hotAddr == addr {
		return
	}
	hotAddr = addr

	tlscfg := &tls.Config{
		ClientSessionCache: tls.NewLRUClientSessionCache(1000),
		NextProtos:         []string{"http/1.1"},
		MinVersion:         tls.VersionTLS13,
		MaxVersion:         tls.VersionTLS13,
	}

	dial := func(a string) (net.Conn, error) {
		t0 := time.Now()
		d := &net.Dialer{
			Timeout:   3 * time.Second,
			KeepAlive: 30 * time.Second,
		}
		c, err := d.Dial("tcp", a)
		if err != nil {
			return nil, err
		}
		if tc, ok := c.(*net.TCPConn); ok {
			_ = tc.SetNoDelay(true)
			_ = tc.SetKeepAlive(true)
			_ = tc.SetKeepAlivePeriod(30 * time.Second)
		}
		// Every occurrence here means fasthttp's pool had no warm connection to
		// reuse and a request is about to pay for a fresh TCP+TLS handshake.
		// Should be rare (startup only) once the pool is kept warm correctly.
		n := atomic.AddInt64(&dialCount, 1)
		fmt.Printf("\x1b[90m[NET] cold TCP connect to %s in %.2fms (dial #%d, TLS handshake follows)\x1b[0m\n",
			a, time.Since(t0).Seconds()*1000, n)
		return c, nil
	}

	hc = &fasthttp.HostClient{
		Addr:                          addr,
		IsTLS:                         true,
		TLSConfig:                     tlscfg,
		Dial:                          dial,
		MaxConns:                      2000,
		MaxIdleConnDuration:           maxIdleConnDuration,
		ReadTimeout:                   5 * time.Second,
		WriteTimeout:                  5 * time.Second,
		MaxResponseBodySize:           64 * 1024,
		DisableHeaderNamesNormalizing: true,
	}
}

func preWarmConns(n int) {
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			warmWithHostClient()
		}()
	}
	wg.Wait()
}

func warmWithHostClient() {
	if hc == nil {
		return
	}
	req := fasthttp.AcquireRequest()
	res := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(res)

	req.Header.SetMethodBytes([]byte("GET"))
	req.SetRequestURIBytes(warmURL)
	_ = hc.Do(req, res)
	res.ResetBody()
}
