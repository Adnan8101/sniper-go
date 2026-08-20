package main

import "github.com/valyala/fasthttp"

var (
	cfg         Config
	cachedProps string
	cachedMFA   string
	hc          *fasthttp.HostClient
)
