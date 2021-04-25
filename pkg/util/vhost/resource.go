// Copyright 2017 fatedier, fatedier@gmail.com
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vhost

import (
	"bytes"
	"io/ioutil"
	"net/http"

	frpLog "github.com/fatedier/frp/pkg/util/log"
	"github.com/fatedier/frp/pkg/util/version"
)

var (
	NotFoundPagePath = ""
)

const (
	NotFound = `<!DOCTYPE html>
<html>
<head>
<title>Not Found</title>
<style>
    body {
        width: 35em;
        margin: 0 auto;
        font-family: Tahoma, Verdana, Arial, sans-serif;
    }
</style>
</head>
<body>
<h1>The page you requested was not found.</h1>
<p>Sorry, the page you are looking for is currently unavailable.<br/>
Please try again later.</p>
<p>The server is powered by <a href="https://github.com/fatedier/frp">frp</a>.</p>
<p><em>Faithfully yours, frp.</em></p>
</body>
</html>
`
	Waiting = `<!DOCTYPE html><html lang=en><meta charset=UTF-8><title>Loading</title><body><style>body,html{height:100%;padding:0;margin:0}
	.wrapper{display:flex;justify-content:center;align-items:center;height:100px}.centerd{display:flex;justify-content:center;align-items:center;height:100%}
	.ball{width:22px;height:22px;border-radius:11px;margin:0 10px;animation:2s bounce ease infinite}.blue{background-color:#4285f5}
	.red{background-color:#ea4436;animation-delay:.25s}.yellow{background-color:#fbbd06;animation-delay:.5s}.green{background-color:#34a952;animation-delay:.75s}
	@keyframes bounce{50%{transform:translateY(25px)}}</style><div class=centerd><div class=wrapper><div class="ball blue"></div><div class="ball red"></div>
	<div class="ball yellow"></div><div class="ball green"></div></div></div><script>var xmlHttp,requestType="";function createXMLHttpRequest()
	{window.ActiveXObject?xmlHttp=new ActiveXObject("Microsoft.XMLHTTP"):window.XMLHttpRequest&&(xmlHttp=new XMLHttpRequest)}
	function doHeaderRequest(e,t){requestType=e,createXMLHttpRequest(),xmlHttp.onreadystatechange=handleStateChange,xmlHttp.open("Head",t,!0),xmlHttp.send(null)}
	function handleStateChange(){4==xmlHttp.readyState&&"isResourceAvailable"==requestType&&getIsResourceAvailable()}function getIsResourceAvailable()
	{521!=xmlHttp.status?window.location.reload(!0):checker=setTimeout(function(){doHeaderRequest("isResourceAvailable",window.location.href)},2e3)}
	var checker=setTimeout(function(){doHeaderRequest("isResourceAvailable",window.location.href)},2e3)</script>`
)

func getNotFoundPageContent() []byte {
	var (
		buf []byte
		err error
	)
	if NotFoundPagePath != "" {
		buf, err = ioutil.ReadFile(NotFoundPagePath)
		if err != nil {
			frpLog.Warn("read custom 404 page error: %v", err)
			buf = []byte(NotFound)
		}
	} else {
		buf = []byte(NotFound)
	}
	return buf
}

func notFoundResponse() *http.Response {
	header := make(http.Header)
	header.Set("server", "frp/"+version.Full())
	header.Set("Content-Type", "text/html")

	res := &http.Response{
		Status:     "Not Found",
		StatusCode: 404,
		Proto:      "HTTP/1.0",
		ProtoMajor: 1,
		ProtoMinor: 0,
		Header:     header,
		Body:       ioutil.NopCloser(bytes.NewReader(getNotFoundPageContent())),
	}
	return res
}

func noAuthResponse() *http.Response {
	header := make(map[string][]string)
	header["WWW-Authenticate"] = []string{`Basic realm="Restricted"`}
	res := &http.Response{
		Status:     "401 Not authorized",
		StatusCode: 401,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     header,
	}
	return res
}

func getWaitingPageContent() []byte {
	return []byte(Waiting)
}

func waitingResponse() *http.Response {
	header := make(http.Header)
	header.Set("server", "frp/"+version.Full())
	header.Set("Content-Type", "text/html")

	res := &http.Response{
		Status:     "Redirecting",
		StatusCode: 521, //Web Server Is Down from cloudflare
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     header,
		Body:       ioutil.NopCloser(bytes.NewReader(getWaitingPageContent())),
	}
	return res
}
