package main

import (
	"strings"
	"sync"

	"github.com/valyala/fasthttp"
)

var (
	cmu         sync.Mutex
	jar         = make(map[string]string)
	cookieDirty = true
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
			cookieDirty = true
		}
	})
}

func flushCookieCache() {
	cmu.Lock()
	defer cmu.Unlock()
	if !cookieDirty {
		return
	}
	if len(jar) == 0 {
		empty := []byte{}
		hotCookiePtr.Store(&empty)
		cookieDirty = false
		return
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
	built := []byte(b.String())
	hotCookiePtr.Store(&built)
	cookieDirty = false
}

// currentCookie returns the cached cookie header built by flushCookieCache.
func currentCookie() []byte {
	if p := hotCookiePtr.Load(); p != nil {
		return *p
	}
	return nil
}

func getCookieHeader() string {
	return string(currentCookie())
}
