package main

import (
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/gorilla/handlers"
)

// singleJoiningSlash is copied from httputil.singleJoiningSlash method.
func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}

// NewSegmentReverseProxy is adapted from the httputil.NewSingleHostReverseProxy
// method, modified to dynamically redirect to different servers (CDN or Tracking API)
// based on the incoming request, and sets the host of the request to the host of of
// the destination URL.
func NewSegmentReverseProxy(cdn *url.URL, trackingAPI *url.URL) http.Handler {
	director := func(req *http.Request) {
		// Figure out which server to redirect to based on the incoming request.
		var target *url.URL
		switch {
		case strings.HasPrefix(req.URL.String(), "/v1/projects"):
			fallthrough
		case strings.HasPrefix(req.URL.String(), "/analytics.js/v1"):
			fallthrough
		case strings.HasPrefix(req.URL.String(), "/next-integrations"):
			fallthrough
		case strings.HasPrefix(req.URL.String(), "/analytics-next/bundles"):
			target = cdn
		default:
			target = trackingAPI
		}

		targetQuery := target.RawQuery
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = singleJoiningSlash(target.Path, req.URL.Path)
		if targetQuery == "" || req.URL.RawQuery == "" {
			req.URL.RawQuery = targetQuery + req.URL.RawQuery
		} else {
			req.URL.RawQuery = targetQuery + "&" + req.URL.RawQuery
		}

		// Set the host of the request to the host of of the destination URL.
		// See http://blog.semanticart.com/blog/2013/11/11/a-proper-api-proxy-written-in-go/.
		req.Host = req.URL.Host
	}

// This is how we could add an allow list for CORS, but we don't need it for now.
// 	mod := func(allowList map[string]bool) func(r *http.Response) error {
//     return func(r *http.Response) error {
// 			if origin := r.Request.Header.Get("Origin"); allowList[origin] {
// 				r.Header.Set("Access-Control-Allow-Origin", origin)
// 				r.Header.Set("Access-Control-Allow-Methods", "GET, POST, HEAD, PUT, OPTIONS")
// 				r.Header.Set("Access-Control-Allow-Headers", "*")
// 				r.Header.Set("Access-Control-Allow-Credentials", "true")
// 			}

	allowList := map[string]bool{
    "*": true,
	}

	mod := func(allowList map[string]bool) func(r *http.Response) error {
    return func(r *http.Response) error {
				r.Header.Set("Access-Control-Allow-Origin", "*")
				r.Header.Set("Access-Control-Allow-Methods", "GET, POST, HEAD, PUT, OPTIONS")
				r.Header.Set("Access-Control-Allow-Headers", "*")
				r.Header.Set("Access-Control-Allow-Credentials", "true")

        return nil
    }
	}

	return &httputil.ReverseProxy{Director: director, ModifyResponse: mod(allowList)}
}

var port = flag.String("port", "8080", "bind address")
var debug = flag.Bool("debug", false, "debug mode")

func main() {
	flag.Parse()
	cdnURL, err := url.Parse("https://cdn.segment.com")
	if err != nil {
		log.Fatal(err)
	}
	trackingAPIURL, err := url.Parse("https://api.segment.io")
	if err != nil {
		log.Fatal(err)
	}
	proxy := NewSegmentReverseProxy(cdnURL, trackingAPIURL)
	if *debug {
		proxy = handlers.LoggingHandler(os.Stdout, proxy)
		log.Printf("serving proxy at port %v\n", *port)
	}

	log.Fatal(http.ListenAndServe(":"+*port, proxy))
}
