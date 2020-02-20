package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptrace"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

func main() {
	fmt.Println("Starting connection pool test")
	bucket := "my-bucket"
	disableSsl := false
	keyPrefix := "test"
	keySize := 1024
	region := "us-east-1"

	transport := &http.Transport{
		Dial: (&net.Dialer{
			DualStack:     true,
			Timeout:       30 * time.Second,
			KeepAlive:     45 * time.Second,
		}).Dial,
		DisableCompression:  false,
		DisableKeepAlives:   false,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     10 * time.Minute,
		TLSHandshakeTimeout: 30 * time.Second}

	client := &http.Client{Timeout: 0, Transport: transport}

	trace := httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			fmt.Println("LocalAddr:", info.Conn.LocalAddr(), ", RemoteAddr:", info.Conn.RemoteAddr(), ", Reused:", info.Reused)
		}}

	config := aws.NewConfig().
		//WithLogLevel(aws.LogDebug).
		WithHTTPClient(client).
		WithMaxRetries(0).
		WithRegion(region).
		WithDisableSSL(disableSsl)

	svc := s3.New(session.New(), config)

	// Ensure that response body is drained
	svc.Handlers.Complete.PushBack(func(req *request.Request) {
		defer req.HTTPResponse.Body.Close()
		io.Copy(ioutil.Discard, req.HTTPResponse.Body)
	})

	for i := 0; i < 101; i++ {
		key := keyPrefix + "_" + strconv.Itoa(i+1)
		buf := make([]byte, keySize, keySize)
		objInput := &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   bytes.NewReader(buf),
		}

		req, _/*resp*/ := svc.PutObjectRequest(objInput)
		req.HTTPRequest = req.HTTPRequest.WithContext(httptrace.WithClientTrace(req.HTTPRequest.Context(), &trace))
		if err := req.Send(); err == nil {
			fmt.Println(key, ": PutObjectRequest succeeded")
		} else {
			fmt.Println(key, ": PutObjectRequest failed")
		}

		time.Sleep(10 * time.Millisecond)
	}

	fmt.Println("Completed connection pool test")
}
