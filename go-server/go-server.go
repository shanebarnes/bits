package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

var (
	mutex    sync.Mutex
	reqCount int64
	timeZero time.Time
)

func incrementRequestCount() int64 {
	mutex.Lock()
	defer mutex.Unlock()
	reqCount++
	return reqCount
}

func launchHttp(addr string, ch chan error) {
	go func() {
		err := http.ListenAndServe(addr, nil)
		ch <- err
	} ()
}

func launchHttps(addr string, ch chan error, cert, key string) {
	go func() {
		err := http.ListenAndServeTLS(addr, cert, key, nil)
		ch <- err
	} ()
}

func printHeaders(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "Request      : %v\n", incrementRequestCount())
	fmt.Fprintf(w, "Timestamp    : %v\n", time.Now().Format(time.RFC3339Nano))
	fmt.Fprintf(w, "Uptime       : %v\n", time.Since(timeZero))
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "Method       : %v\n", req.Method)
	fmt.Fprintf(w, "URL          : %v\n", req.RequestURI)
	fmt.Fprintf(w, "Protocol     : %v\n", req.Proto)
	fmt.Fprintf(w, "ContentLength: %v\n", req.ContentLength)
	fmt.Fprintf(w, "Host         : %v\n", req.Host)
	fmt.Fprintf(w, "RemoteAddress: %v\n", req.RemoteAddr)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "Headers      :\n")
	fmt.Fprintf(w, "\n")

	for name, headers := range req.Header {
		for _, h := range headers {
			fmt.Fprintf(w, "    %-32v: %v\n", name, h)
		}
	}
}

func main() {
	timeZero = time.Now()

	addrHttp := flag.String("http", ":80", "HTTP server listen address")
	addrHttps := flag.String("https", ":443", "HTTPS server listen address")
	certFile := flag.String("cert", "", "HTTPS certificate file")
	keyFile := flag.String("key", "", "HTTPS private key file")
	flag.Parse()

	chHttp := make(chan error)
	chHttps := make(chan error)

	http.HandleFunc("/", printHeaders)

	fmt.Fprintf(os.Stdout, "HTTP Server: starting\n")
	launchHttp(*addrHttp, chHttp)

	fmt.Fprintf(os.Stdout, "HTTPS Server: starting\n")
	launchHttps(*addrHttps, chHttps, *certFile, *keyFile)

	select {
	case err := <-chHttp:
		fmt.Fprintf(os.Stderr, "HTTP Server: %v\n", err)
	case err := <-chHttps:
		fmt.Fprintf(os.Stderr, "HTTPS Server: %v\n", err)
	}
}
