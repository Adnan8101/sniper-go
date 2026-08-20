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
	hotURL        []byte
	hotReferer    []byte
	hotOrigin     []byte
	hotToken      []byte
	hotSuperProp  []byte
	hotUserAgent  []byte
	hotClientAddr string
)

var (
	hkMethod          = []byte("PATCH")
	hkAuthorization   = []byte("Authorization")
	hkUserAgent       = []byte("User-Agent")
	hkXSuperProps     = []byte("X-Super-Properties")
	hkContentType     = []byte("Content-Type")
	hkAcceptEncoding  = []byte("Accept-Encoding")
	hkOrigin          = []byte("Origin")
	hkReferer         = []byte("Referer")
	hkMFAAuth         = []byte("X-Discord-MFA-Authorization")
	hkCookie          = []byte("Cookie")
	hvContentType    = []byte("application/json")
	hvAcceptEncoding = []byte("identity")
)

var bodyBuf [64]byte

// buildBody writes {"code":"<vanity>"} into bodyBuf and returns the byte slice.
func buildBody(vanity string) []byte {
	n := copy(bodyBuf[:], `{"code":"`)
	n += copy(bodyBuf[n:], vanity)
	n += copy(bodyBuf[n:], `"}`)
	return bodyBuf[:n]
}

// rebuildHotCache pre-computes static request bytes and refreshes the HostClient.
func rebuildHotCache() {
	host := config.GetHost()
	apiVer := config.GetAPIVersion()
	guildID := config.GuildID
	if guildID == "" {
		guildID = "1539670174221864963"
	}

	hotURL = []byte(fmt.Sprintf("https://%s/api/%s/guilds/%s/vanity-url", host, apiVer, guildID))
	hotReferer = []byte(fmt.Sprintf("https://%s/channels/%s", host, guildID))
	hotOrigin = []byte("https://" + host)
	hotToken = []byte(config.GetToken())
	hotSuperProp = []byte(config.BuildSuperProperties())
	hotUserAgent = []byte(config.UserAgent())

	initHostClient(host)
}

// initHostClient initializes the fasthttp HostClient for the given host.
func initHostClient(host string) {
	newAddr := host + ":443"
	if hostClient != nil && hotClientAddr == newAddr {
		return
	}
	hotClientAddr = newAddr

	tlsCfg := &tls.Config{
		InsecureSkipVerify:     true,
		SessionTicketsDisabled: false,
		ClientSessionCache:     tls.NewLRUClientSessionCache(1000),
		NextProtos:             []string{"http/1.1"},
		MinVersion:             tls.VersionTLS13,
		MaxVersion:             tls.VersionTLS13,
	}

	dialFn := func(addr string) (net.Conn, error) {
		d := &net.Dialer{
			Timeout:   2 * time.Second,
			KeepAlive: 15 * time.Second,
		}
		conn, err := d.Dial("tcp", addr)
		if err != nil {
			return nil, err
		}
		if tc, ok := conn.(*net.TCPConn); ok {
			_ = tc.SetNoDelay(true)
			_ = tc.SetKeepAlive(true)
			_ = tc.SetKeepAlivePeriod(15 * time.Second)
			_ = tc.SetReadBuffer(256 * 1024)
			_ = tc.SetWriteBuffer(256 * 1024)
		}
		return conn, nil
	}

	hostClient = &fasthttp.HostClient{
		Addr:                          host + ":443",
		IsTLS:                         true,
		TLSConfig:                     tlsCfg,
		Dial:                          dialFn,
		MaxConns:                      2000,
		MaxIdleConnDuration:           1200 * time.Second,
		ReadTimeout:                   2 * time.Second,
		WriteTimeout:                  2 * time.Second,
		MaxResponseBodySize:           256 * 1024,
		DisableHeaderNamesNormalizing: true,
	}
}

// preWarmConns warms up active pool connections in parallel.
func preWarmConns(count int) {
	var wg sync.WaitGroup
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			defer wg.Done()
			warmWithHostClient()
		}()
	}
	wg.Wait()
}

// warmWithHostClient sends a pre-heating GET request over the hostClient.
func warmWithHostClient() {
	if hostClient == nil {
		return
	}
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.Header.SetMethodBytes([]byte("GET"))
	req.SetRequestURI(fmt.Sprintf("https://%s/api/v9/gateway", config.GetHost()))
	_ = hostClient.Do(req, resp)
	resp.ResetBody()
}
