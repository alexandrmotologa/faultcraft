package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// DemoUpstream starts lightweight mock servers simulating PostgreSQL, Redis, and HTTP services.
func main() {
	fmt.Println("🚀 Starting FaultCraft Demo Upstream Services...")

	// 1. Mock Redis (RESP) on :6379 or fallback :16379
	redisLn, redisPort := startListener(6379, 16379)
	go serveMockRedis(redisLn)
	fmt.Printf("   [Mock Redis]      Listening on :%d\n", redisPort)

	// 2. Mock HTTP API on :8080 or fallback :18080
	httpLn, httpPort := startListener(8080, 18080)
	go serveMockHTTP(httpLn)
	fmt.Printf("   [Mock HTTP API]   Listening on :%d\n", httpPort)

	// 3. Mock PostgreSQL Echo on :5432 or fallback :15432
	echoLn, echoPort := startListener(5432, 25432)
	go serveMockTCP(echoLn)
	fmt.Printf("   [Mock TCP/Echo]   Listening on :%d\n\n", echoPort)

	fmt.Println("Press Ctrl+C to terminate upstream services.")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	fmt.Println("Shutting down mock upstreams.")
}

func startListener(preferredPort, fallbackPort int) (net.Listener, int) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", preferredPort))
	if err == nil {
		return ln, preferredPort
	}
	ln, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", fallbackPort))
	if err == nil {
		return ln, fallbackPort
	}
	ln, _ = net.Listen("tcp", "127.0.0.1:0")
	return ln, ln.Addr().(*net.TCPAddr).Port
}

func serveMockRedis(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			defer c.Close()
			scanner := bufio.NewScanner(c)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.EqualFold(line, "PING") || strings.Contains(strings.ToUpper(line), "PING") {
					_, _ = c.Write([]byte("+PONG\r\n"))
				} else {
					_, _ = c.Write([]byte("+OK\r\n"))
				}
			}
		}(conn)
	}
}

func serveMockHTTP(ln net.Listener) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy","uptime_sec":1234}`))
	})
	mux.HandleFunc("/checkout", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"order_id":"ord_9988","status":"authorized","amount":99.00}`))
	})
	_ = http.Serve(ln, mux)
}

func serveMockTCP(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			defer c.Close()
			buf := make([]byte, 2048)
			for {
				n, err := c.Read(buf)
				if n > 0 {
					// Echo back with ack prefix
					out := bytes.Join([][]byte{[]byte("ACK:"), buf[:n]}, nil)
					_, _ = c.Write(out)
				}
				if err != nil {
					return
				}
			}
		}(conn)
	}
}
