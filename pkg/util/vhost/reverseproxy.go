// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// HTTP reverse proxy handler

package vhost

import (
	"context"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type ReverseProxy struct {
	*httputil.ReverseProxy
}

func NewSingleHostReverseProxy(target *url.URL) *ReverseProxy {
	rp := &ReverseProxy{
		ReverseProxy: httputil.NewSingleHostReverseProxy(target),
	}
	rp.ErrorHandler = rp.errHandler
	return rp
}

func NewReverseProxy(orp *httputil.ReverseProxy) *ReverseProxy {
	rp := &ReverseProxy{
		ReverseProxy: orp,
	}
	rp.ErrorHandler = rp.errHandler
	return rp
}

func (p *ReverseProxy) errHandler(rw http.ResponseWriter, r *http.Request, e error) {
	if e == io.EOF {
		rw.WriteHeader(521)
		rw.Write(getWaitingPageContent())
	} else {
		rw.WriteHeader(http.StatusNotFound)
		rw.Write(getNotFoundPageContent())
	}
}

func (p *ReverseProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Modify for frp
	req = req.WithContext(context.WithValue(req.Context(), RouteInfoURL, req.URL.Path))
	req = req.WithContext(context.WithValue(req.Context(), RouteInfoHost, req.Host))
	req = req.WithContext(context.WithValue(req.Context(), RouteInfoRemote, req.RemoteAddr))

	p.ReverseProxy.ServeHTTP(rw, req)
}
