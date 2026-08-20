package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

var (
	hotURL, hotRef, hotOrigin, hotTok, hotProp, hotUA []byte
	hotAddr                                           string
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
	return []byte(`{"code":"` + code + `"}`)
}

func rebuildHotCache() {
	h := cfg.GetHost()
	ver := cfg.GetAPIVersion()
	gid := cfg.GuildID
	if gid == "" {
		gid = "1539670174221864963"
	}

	hotURL = []byte(fmt.Sprintf("https://%s/api/%s/guilds/%s/vanity-url", h, ver, gid))
	hotRef = []byte(fmt.Sprintf("https://%s/channels/%s", h, gid))
	hotOrigin = []byte("https://" + h)
	hotTok = []byte(cfg.GetToken())
	hotProp = []byte(cfg.BuildSuperProperties())
	hotUA = []byte(cfg.UserAgent())

	initHostClient(h)
}

func initHostClient(h string) {
	addr := h + ":443"
	if hc != nil && hotAddr == addr {
		return
	}
	hotAddr = addr

	tcfg := &tls.Config{
		ClientSessionCache: tls.NewLRUClientSessionCache(1000),
		NextProtos:         []string{"http/1.1"},
		MinVersion:         tls.VersionTLS13,
		MaxVersion:         tls.VersionTLS13,
	}

	dial := func(a string) (net.Conn, error) {
		d := &net.Dialer{
			Timeout:   2 * time.Second,
			KeepAlive: 15 * time.Second,
		}
		c, err := d.Dial("tcp", a)
		if err != nil {
			return nil, err
		}
		if tc, ok := c.(*net.TCPConn); ok {
			_ = tc.SetNoDelay(true)
			_ = tc.SetKeepAlive(true)
			_ = tc.SetKeepAlivePeriod(15 * time.Second)
			_ = tc.SetReadBuffer(256 * 1024)
			_ = tc.SetWriteBuffer(256 * 1024)
		}
		return c, nil
	}

	hc = &fasthttp.HostClient{
		Addr:                          addr,
		IsTLS:                         true,
		TLSConfig:                     tcfg,
		Dial:                          dial,
		MaxConns:                      2000,
		MaxIdleConnDuration:           1200 * time.Second,
		ReadTimeout:                   2 * time.Second,
		WriteTimeout:                  2 * time.Second,
		MaxResponseBodySize:           256 * 1024,
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
	req.SetRequestURI(fmt.Sprintf("https://%s/api/v9/gateway", cfg.GetHost()))
	_ = hc.Do(req, res)
	res.ResetBody()
}
