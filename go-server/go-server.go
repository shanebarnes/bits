package main

import (
	"context"
	"crypto/tls"
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

	quic "github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/http3"
	gttls "github.com/shanebarnes/goto/crypto/tls"
	"golang.org/x/net/http2"
	"golang.org/x/net/netutil"
)

const (
	networkTcp    = "tcp"
	networkUdp    = "udp"
	schemeHttp    = "http"
	schemeHttps   = "https"
	tlsMinVersion = tls.VersionTLS12
)

var (
	addrs    arrStr
	backlog  int
	mp       map[string]int
	mu       sync.Mutex
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
	mu.Lock()
	defer mu.Unlock()
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

	mu.Lock()
	switch state {
	case http.StateNew:
		if len(mp) > backlog {
			conn.Close()
		} else if _, ok := mp[remoteAddr]; !ok {
			mp[remoteAddr] = 1
		} else {
			conn.Close()
		}
	case http.StateHijacked, http.StateClosed:
		delete(mp, remoteAddr)
	}
	mu.Unlock()

	fmt.Fprintf(os.Stdout,
		"server %v: remote address=%v, http state=%v\n",
		localAddr,
		remoteAddr,
		state.String())
}

func listenConfig() *net.ListenConfig {
	return &net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			return nil
		},
	}
}

func newHttpServer(tlsConfig *tls.Config) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", printRequestTrace)
	server := http.Server{ConnState: connStateCb, Handler: mux, TLSConfig: tlsConfig}
	server.SetKeepAlivesEnabled(false)
	http2.ConfigureServer(&server, &http2.Server{MaxConcurrentStreams: 100})
	return &server
}

func newTlsConfig(reloader *gttls.Reloader) *tls.Config {
	return &tls.Config{
		GetCertificate: reloader.GetCertificateFunc(),
		MinVersion:     tlsMinVersion,
		NextProtos:     []string{"h2", "http/1.1"},
	}
}

func serveHttp(network, addr, tlsCert, tlsKey string, ch chan error) {
	go func() {
		fmt.Fprintf(os.Stdout, "server %v: starting on %v\n", addr, network)
		url, err := url.Parse(addr)
		if err == nil {
			switch network {
			case networkUdp:
				err = serveHttpUdp(url, tlsCert, tlsKey)
			default:
				err = serveHttpTcp(url, tlsCert, tlsKey)
			}
		}
		fmt.Fprintf(os.Stdout, "server %v: stopping on %v, %v\n", addr, network, err)
		ch <- err
	}()
}

func serveHttpTcp(url *url.URL, tlsCert, tlsKey string) error {
	if reloader, err := gttls.NewCertificateReloader(tlsCert, tlsKey); err != nil {
		return err
	} else if listener, err := listenConfig().Listen(context.Background(), networkTcp, url.Host); err != nil {
		reloader.Close()
		return err
	} else {
		listener = netutil.LimitListener(listener, syscall.SOMAXCONN)
		if url.Scheme == schemeHttps {
			listener = tls.NewListener(listener, newTlsConfig(reloader))
		}

		defer listener.Close()
		defer reloader.Close()

		switch url.Scheme {
		case schemeHttp, schemeHttps:
			return newHttpServer(newTlsConfig(reloader)).Serve(listener)
		default:
			return syscall.EINVAL
		}
	}
}

func serveHttpUdp(url *url.URL, tlsCert, tlsKey string) error {
	if reloader, err := gttls.NewCertificateReloader(tlsCert, tlsKey); err != nil {
		return err
	} else if pc, err := listenConfig().ListenPacket(context.Background(), networkUdp, url.Host); err != nil {
		reloader.Close()
		return err
	} else {
		defer pc.Close()
		defer reloader.Close()

		server := http3.Server{
			QuicConfig: &quic.Config{
				KeepAlive:             false,
				MaxIncomingUniStreams: 0,
				MaxIncomingStreams:    0,
				MaxIdleTimeout:        10 * time.Second,
			},
			Server: newHttpServer(newTlsConfig(reloader)),
		}

		return server.Serve(pc)
	}
}

type tlsTrace struct {
	Version                    string `json:"version"`
	HandshakeComplete          bool   `json:"handshakeComplete"`
	DidResume                  bool   `json:"didResume"`
	CipherSuite                string `json:"cipherSuite"`
	NegotiatedProtocol         string `json:"negotiatedProtocol"`
	NegotiatedProtocolIsMutual bool   `json:"negotiatedProtocolIsMutual"`
	ServerName                 string `json:"serverName"`
}

type requestTrace struct {
	Id            int64       `json:"traceId"`
	Time          string      `json:"time"`
	Uptime        string      `json:"uptime"`
	Tls           tlsTrace    `json:"tlsConnectionState"`
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
	for _, writer := range writers {
		data := requestTrace{
			Id:            traceId,
			Time:          now.Format(time.RFC3339Nano),
			Uptime:        now.Sub(timeZero).String(),
			Tls:           getTlsTrace(req),
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

func getTlsVersion(version uint16) string {
	str := ""
	switch version {
	case tls.VersionSSL30:
		str = "SSL 3.0"
	case tls.VersionTLS10:
		str = "TLS 1.0"
	case tls.VersionTLS11:
		str = "TLS 1.1"
	case tls.VersionTLS12:
		str = "TLS 1.2"
	case tls.VersionTLS13:
		str = "TLS 1.3"
	default:
		str = fmt.Sprintf("0x%04X", version)
	}
	return str
}

func getTlsTrace(req *http.Request) tlsTrace {
	if req.TLS != nil {
		return tlsTrace{
			Version:                    getTlsVersion(req.TLS.Version),
			HandshakeComplete:          req.TLS.HandshakeComplete,
			DidResume:                  req.TLS.DidResume,
			CipherSuite:                tls.CipherSuiteName(req.TLS.CipherSuite),
			NegotiatedProtocol:         req.TLS.NegotiatedProtocol,
			NegotiatedProtocolIsMutual: req.TLS.NegotiatedProtocolIsMutual,
			ServerName:                 req.TLS.ServerName,
		}
	}
	return tlsTrace{}
}

func init() {
	mp = make(map[string]int)
}

func main() {
	timeZero = time.Now()

	flag.Var(&addrs, "addr", "Server listen address (e.g., https://:80)")
	flag.IntVar(&backlog, "backlog", 10, "Maximum number of connection requests queued")
	certFile := flag.String("cert", "", "TLS certificate file")
	keyFile := flag.String("key", "", "TLS private key file")
	flag.Parse()

	if len(addrs) > 0 {
		chErr := make(chan error)

		for _, addr := range addrs {
			if url, err := url.Parse(addr); err == nil && url.Scheme == schemeHttps {
				serveHttp(networkUdp, addr, *certFile, *keyFile, chErr)
			}
			serveHttp(networkTcp, addr, *certFile, *keyFile, chErr)
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
