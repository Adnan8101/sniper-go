package main

import (
	"sync/atomic"

	"github.com/valyala/fasthttp"
)

var (
	cfg         Config
	cachedProps string
	cachedMFA   string
	hc          *fasthttp.HostClient
)

var (
	hotMFAPtr    atomic.Pointer[[]byte]
	hotCookiePtr atomic.Pointer[[]byte]
)

var (
	dialCount int64
	fireCount int64
)
