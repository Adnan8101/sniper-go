package main

import (
	"strings"
	"sync"

	"github.com/valyala/fasthttp"
)

var (
	cmu sync.Mutex
	jar = make(map[string]string)
)

func absorbCookies(res *fasthttp.Response) {
	cmu.Lock()
	defer cmu.Unlock()
	res.Header.VisitAllCookie(func(k, v []byte) {
		c := fasthttp.AcquireCookie()
		c.ParseBytes(v)
		key := string(c.Key())
		val := string(c.Value())
		fasthttp.ReleaseCookie(c)
		if key != "" {
			jar[key] = val
		}
	})
}

func getCookieHeader() string {
	cmu.Lock()
	defer cmu.Unlock()
	if len(jar) == 0 {
		return ""
	}
	var b strings.Builder
	first := true
	for k, v := range jar {
		if !first {
			b.WriteString("; ")
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(v)
		first = false
	}
	return b.String()
}
