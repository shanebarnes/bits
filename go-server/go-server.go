package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/netutil"
)

const (
	schemeHttp  = "http"
	schemeHttps = "https"
)

var (
	mutex    sync.Mutex
	reqCount int64
	timeZero time.Time
)

type arrStr []string

func (a *arrStr) String() string {
	return strings.Join(*a, ", ")
}

func (a *arrStr) Set(val string) error {
	*a = append(*a, val)
	return nil
}

func incrementRequestCount() int64 {
	mutex.Lock()
	defer mutex.Unlock()
	reqCount++
	return reqCount
}

func connStateCb(conn net.Conn, state http.ConnState) {
	var localAddr, remoteAddr string
	if conn.LocalAddr() != nil {
		localAddr = conn.LocalAddr().String()
	}

	if conn.RemoteAddr() != nil {
		remoteAddr = conn.RemoteAddr().String()
	}

	fmt.Fprintf(os.Stdout,
		"server %v: remote address=%v, http state=%v\n",
		localAddr,
		remoteAddr,
		state.String())
}

func newHttpServer() *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", printRequestTrace)
	return &http.Server{ConnState: connStateCb, Handler: mux}
}

func serveHttp(addr, tlsCert, tlsKey string, ch chan error) {
	go func() {
		fmt.Fprintf(os.Stdout, "server %v: starting\n", addr)
		url, err := url.Parse(addr)
		if err == nil {
			var listener net.Listener
			listener, err = net.Listen("tcp", url.Host)
			if err == nil {
				listener = netutil.LimitListener(listener, syscall.SOMAXCONN)
				defer listener.Close()
				switch url.Scheme {
				case schemeHttp:
					err = newHttpServer().Serve(listener)
				case schemeHttps:
					err = newHttpServer().ServeTLS(listener, tlsCert, tlsKey)
				default:
					err = syscall.EINVAL
				}
			}
		}
		fmt.Fprintf(os.Stdout, "server %v: stopping, %v\n", addr, err)
		ch <- err
	} ()
}

type requestTrace struct {
	Id            int64       `json:"traceId"`
	Time          string      `json:"time"`
	Uptime        string      `json:"uptime"`
	Method        string      `json:"method"`
	Url           string      `json:"url"`
	Protocol      string      `json:"protocol"`
	ContentLength int64       `json:"contentLength"`
	Host          string      `json:"host"`
	RemoteAddress string      `json:"remoteAddress"`
	Headers       http.Header `json:"headers"`
}

func printRequestTrace(rw http.ResponseWriter, req *http.Request) {
	now := time.Now()
	traceId := incrementRequestCount()
	writers := []io.Writer{os.Stdout, rw}
	for _, writer := range writers{
		data := requestTrace{
			Id:            traceId,
			Time:          now.Format(time.RFC3339Nano),
			Uptime:        now.Sub(timeZero).String(),
			Method:        req.Method,
			Url:           req.RequestURI,
			Protocol:      req.Proto,
			ContentLength: req.ContentLength,
			Host:          req.Host,
			RemoteAddress: req.RemoteAddr,
			Headers:       req.Header,
		}
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "   ") // Make it pretty
		encoder.Encode(data)
	}
}

func main() {
	timeZero = time.Now()

	var addrs arrStr
	flag.Var(&addrs, "addr", "Server listen address (e.g., https://:80)")
	certFile := flag.String("cert", "", "TLS certificate file")
	keyFile := flag.String("key", "", "TLS private key file")
	flag.Parse()

	if len(addrs) > 0 {
		chErr := make(chan error)

		for _, addr := range addrs {
			serveHttp(addr, *certFile, *keyFile, chErr)
		}

		// Exit on first error
		select {
		case <-chErr:
		}
	} else {
		fmt.Fprintf(os.Stdout, "Usage of go-server:\n\n")
		flag.PrintDefaults()
	}
}
