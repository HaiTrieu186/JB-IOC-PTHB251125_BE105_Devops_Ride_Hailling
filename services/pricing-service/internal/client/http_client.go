package client

import (
	"net"
	"net/http"
	"time"
)

// SharedHTTPClient là singleton http.Client cho toàn bộ giao tiếp nội bộ và OSRM
// Thiết lập MaxIdleConns = 100 và MaxIdleConnsPerHost = 50 theo Mục 5(k) LLD-00.
var SharedHTTPClient = &http.Client{
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 50,
		IdleConnTimeout:     90 * time.Second,
	},
}
