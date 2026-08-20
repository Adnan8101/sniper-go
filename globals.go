package main
import "github.com/valyala/fasthttp"
var (
	config                Config
	cachedSuperProperties string
	cachedMfaToken        string
	hostClient            *fasthttp.HostClient
)
