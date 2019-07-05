// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// HTTP reverse proxy handler

package vhost

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	frpIo "github.com/fatedier/golib/io"
)

type ReverseProxy struct {
	*httputil.ReverseProxy
	WebSocketDialContext func(ctx context.Context, network, addr string) (net.Conn, error)
}

func IsWebsocketRequest(req *http.Request) bool {
	containsHeader := func(name, value string) bool {
		items := strings.Split(req.Header.Get(name), ",")
		for _, item := range items {
			if value == strings.ToLower(strings.TrimSpace(item)) {
				return true
			}
		}
		return false
	}
	return containsHeader("Connection", "upgrade") && containsHeader("Upgrade", "websocket")
}

func NewSingleHostReverseProxy(target *url.URL) *ReverseProxy {
	rp := &ReverseProxy{
		ReverseProxy:         httputil.NewSingleHostReverseProxy(target),
		WebSocketDialContext: nil,
	}
	rp.ErrorHandler = rp.errHandler
	return rp
}

func NewReverseProxy(orp *httputil.ReverseProxy) *ReverseProxy {
	rp := &ReverseProxy{
		ReverseProxy:         orp,
		WebSocketDialContext: nil,
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
	req = req.WithContext(context.WithValue(req.Context(), "url", req.URL.Path))
	req = req.WithContext(context.WithValue(req.Context(), "host", req.Host))
	req = req.WithContext(context.WithValue(req.Context(), "remote", req.RemoteAddr))

	if IsWebsocketRequest(req) {
		p.serveWebSocket(rw, req)
	} else {
		p.ReverseProxy.ServeHTTP(rw, req)
	}
}

func (p *ReverseProxy) serveWebSocket(rw http.ResponseWriter, req *http.Request) {
	if p.WebSocketDialContext == nil {
		rw.WriteHeader(500)
		return
	}
	targetConn, err := p.WebSocketDialContext(req.Context(), "tcp", "")
	if err != nil {
		rw.WriteHeader(501)
		return
	}
	defer targetConn.Close()

	p.Director(req)

	hijacker, ok := rw.(http.Hijacker)
	if !ok {
		rw.WriteHeader(500)
		return
	}
	conn, _, errHijack := hijacker.Hijack()
	if errHijack != nil {
		rw.WriteHeader(500)
		return
	}
	defer conn.Close()

	req.Write(targetConn)
	frpIo.Join(conn, targetConn)
}
