package main
import (
	"strings"
	"sync"
	"github.com/valyala/fasthttp"
)
var (
	cookieMu    sync.Mutex
	cookieStore = make(map[string]string)
)
func absorbCookies(resp *fasthttp.Response) {
	cookieMu.Lock()
	defer cookieMu.Unlock()
	resp.Header.VisitAllCookie(func(key, value []byte) {
		c := fasthttp.AcquireCookie()
		c.ParseBytes(value)
		k := string(c.Key())
		v := string(c.Value())
		fasthttp.ReleaseCookie(c)
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
	var b strings.Builder
	first := true
	for k, v := range cookieStore {
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
