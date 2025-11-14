package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
)

// Start SNI Proxy
func startSniProxy() {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// If listen_addr_http is configured, start an HTTP listener (for 301 redirect to HTTPS)
	if cfg.ListenAddrHTTP != "" {
		listenerHTTP, err := net.Listen("tcp", cfg.ListenAddrHTTP)
		if err != nil {
			serviceLogger(fmt.Sprintf("Failed to listen (HTTP) on %s: %v", cfg.ListenAddrHTTP, err), 31, false)
			os.Exit(1)
		}
		serviceLogger(fmt.Sprintf("Listening on: %v", listenerHTTP.Addr()), 0, false)

		go func(listenerHTTP net.Listener) {
			defer listenerHTTP.Close()
			for {
				conn, err := listenerHTTP.Accept()
				if err != nil {
					serviceLogger(fmt.Sprintf("Error while accepting (HTTP) connection: %v", err), 31, false)
					continue
				}
				raddr := conn.RemoteAddr().(*net.TCPAddr)
				serviceLogger("Connection from: "+raddr.String(), 32, false)
				go handleHTTPRedirect(conn) // New connection; handle in a new goroutine
			}
		}(listenerHTTP)
	}

	// Start HTTPS listener
	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		serviceLogger(fmt.Sprintf("Failed to listen (HTTPS) on %s: %v", cfg.ListenAddr, err), 31, false)
		os.Exit(1)
	}
	serviceLogger(fmt.Sprintf("Listening on: %v", listener.Addr()), 0, false)

	go func(listener net.Listener) {
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			if err != nil {
				serviceLogger(fmt.Sprintf("Error while accepting (HTTPS) connection: %v", err), 31, false)
				continue
			}
			raddr := conn.RemoteAddr().(*net.TCPAddr)
			serviceLogger("Connection from: "+raddr.String(), 32, false)
			go serve(conn, raddr.String()) // New connection; handle in a new goroutine
		}
	}(listener)

	// Exit when a termination signal is received
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	s := <-ch
	cancel()
	fmt.Printf("\nSignal received %s, Exit.\n", s)
}

// forward forwards initial data to dst and then pipes traffic between client and backend.
// It dials dst using GetDialer, writes data to backend, and connects streams via ioReflector.
func forward(conn net.Conn, data []byte, dst string, raddr string) {
	backend, err := GetDialer(cfg.EnableSocks).Dial("tcp", dst)
	if err != nil {
		serviceLogger(fmt.Sprintf("Unable to connect to backend (HTTPS): %v", err), 31, false)
		return
	}

	defer backend.Close()

	if _, err = backend.Write(data); err != nil {
		serviceLogger(fmt.Sprintf("Unable to forward to backend (HTTPS): %v", err), 31, false)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conChk := make(chan struct{})
	go ioReflector(ctx, backend, conn, false, conChk, raddr, dst)
	go ioReflector(ctx, conn, backend, true, conChk, raddr, dst)
	<-conChk
	// Cancel context to notify the other ioReflector to exit
	cancel()
}

