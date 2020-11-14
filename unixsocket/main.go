package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
)

const (
	sockName = "unix.sock"
)

func main() {
	server := flag.Bool("s", false, "server mode")
	flag.Parse()

	sockAddr := os.TempDir() + sockName
	if *server {
		runServer(sockAddr)
	} else {
		runClient(sockAddr)
	}
}

func runClient(sockAddr string) {
	fmt.Println("Running in client mode")
	if conn, err := net.Dial("unix", sockAddr); err == nil {
		defer conn.Close()
		fmt.Fprintf(os.Stdout, "Connected to %s\n", sockAddr)
		msg := "hello, world!\n"
		if nw, err := conn.Write([]byte(msg)); err == nil {
			fmt.Fprintf(os.Stdout, "Wrote %d bytes: %s\n", nw, msg)
		} else {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func runServer(sockAddr string) {
	fmt.Println("Running in server mode")

	var err error
	var ln net.Listener
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		fmt.Fprintf(os.Stderr, "\nCaught signal: %v\n", sig)
		if ln != nil {
			fmt.Fprintf(os.Stderr, "Closing %s\n", sockAddr)
			ln.Close()
		}
		done <- true
	} ()

	if ln, err = net.Listen("unix", sockAddr); err == nil {
		defer ln.Close()
		fmt.Fprintf(os.Stdout, "Listening on %s\n", sockAddr)

		for err == nil {
			var conn net.Conn
			conn, err = ln.Accept()
			if err == nil {
				go handleAccept(conn)
			}
		}

		fmt.Fprintf(os.Stderr, "%v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func handleAccept(c net.Conn) {
	defer c.Close()
	buf := make([]byte, 1024)
	for {
		if nr, err := c.Read(buf); err == nil {
			fmt.Fprintf(os.Stdout, "Read %d bytes: %s\n", nr, string(buf[:nr]))
		} else {
			return
		}
	}
}
