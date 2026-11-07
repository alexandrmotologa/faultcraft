package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

func startPostgresMock(addr string) net.Listener {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 4096)
				for {
					c.SetReadDeadline(time.Now().Add(10 * time.Second))
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					data := buf[:n]
					// If StartupMessage (contains \x00\x03\x00\x00)
					if len(data) >= 8 && data[4] == 0 && data[5] == 3 {
						// AuthOK ('R', len=8, code=0) + ReadyForQuery ('Z', len=5, status='I')
						authOk := []byte{'R', 0, 0, 0, 8, 0, 0, 0, 0}
						ready := []byte{'Z', 0, 0, 0, 5, 'I'}
						c.Write(append(authOk, ready...))
						continue
					}
					// If Simple Query ('Q')
					if len(data) > 0 && data[0] == 'Q' {
						// RowDescription ('T') + DataRow ('D') + CommandComplete ('C') + ReadyForQuery ('Z')
						resp := bytes.Buffer{}
						// RowDescription: 1 col named "status"
						colName := "result\x00"
						tLen := 4 + 2 + len(colName) + 4 + 2 + 4 + 2 + 4 + 2
						resp.WriteByte('T')
						resp.Write([]byte{0, 0, byte(tLen >> 8), byte(tLen)})
						resp.Write([]byte{0, 1}) // 1 column
						resp.WriteString(colName)
						resp.Write([]byte{0, 0, 0, 0, 0, 1, 0, 0, 0, 25, 0, 0, 0, 0, 0, 0})

						// DataRow: value "1"
						val := "OK"
						dLen := 4 + 2 + 4 + len(val)
						resp.WriteByte('D')
						resp.Write([]byte{0, 0, byte(dLen >> 8), byte(dLen)})
						resp.Write([]byte{0, 1}) // 1 field
						resp.Write([]byte{0, 0, 0, byte(len(val))})
						resp.WriteString(val)

						// CommandComplete: SELECT 1
						cmd := "SELECT 1\x00"
						cLen := 4 + len(cmd)
						resp.WriteByte('C')
						resp.Write([]byte{0, 0, byte(cLen >> 8), byte(cLen)})
						resp.WriteString(cmd)

						// ReadyForQuery
						resp.Write([]byte{'Z', 0, 0, 0, 5, 'I'})
						c.Write(resp.Bytes())
						continue
					}
					// Default echo / ack
					c.Write([]byte{'Z', 0, 0, 0, 5, 'I'})
				}
			}(conn)
		}
	}()
	return l
}

func startRedisMock(addr string) net.Listener {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 4096)
				for {
					c.SetReadDeadline(time.Now().Add(10 * time.Second))
					n, err := c.Read(buf)
					if err != nil {
						return
					}
					req := string(buf[:n])
					if strings.Contains(strings.ToUpper(req), "PING") {
						c.Write([]byte("+PONG\r\n"))
					} else if strings.Contains(strings.ToUpper(req), "GET") {
						c.Write([]byte("$18\r\n{\"user_id\":\"u_99\"}\r\n"))
					} else if strings.Contains(strings.ToUpper(req), "SET") {
						c.Write([]byte("+OK\r\n"))
					} else {
						c.Write([]byte("+OK\r\n"))
					}
				}
			}(conn)
		}
	}()
	return l
}

func startHTTPMock(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"authorized","transaction_id":"tx_884920","amount":4900,"currency":"usd"}`))
	})
	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		srv.ListenAndServe()
	}()
	return srv
}

func setupProxiesAndToxics(apiBase string) {
	client := &http.Client{Timeout: 5 * time.Second}

	// 1. Create postgres_production
	createProxy(client, apiBase, map[string]interface{}{
		"name":     "postgres_production",
		"listen":   "127.0.0.1:5433",
		"upstream": "127.0.0.1:15432",
		"enabled":  true,
	})

	// 2. Create redis_cache
	createProxy(client, apiBase, map[string]interface{}{
		"name":     "redis_cache",
		"listen":   "127.0.0.1:6380",
		"upstream": "127.0.0.1:16379",
		"enabled":  true,
	})

	// 3. Create stripe_payment_api
	createProxy(client, apiBase, map[string]interface{}{
		"name":     "stripe_payment_api",
		"listen":   "127.0.0.1:8081",
		"upstream": "127.0.0.1:18080",
		"enabled":  true,
	})

	// Add toxics
	addToxic(client, apiBase, "postgres_production", map[string]interface{}{
		"name":      "latency_downstream",
		"type":      "latency",
		"stream":    "downstream",
		"toxicity":  1.0,
		"auto_heal": 300,
		"attributes": map[string]interface{}{
			"latency": 115,
			"jitter":  20,
		},
	})

	addToxic(client, apiBase, "postgres_production", map[string]interface{}{
		"name":      "slicer_upstream",
		"type":      "slicer",
		"stream":    "upstream",
		"toxicity":  1.0,
		"auto_heal": 300,
		"attributes": map[string]interface{}{
			"average_size": 128,
			"delay":        2000,
		},
	})

	addToxic(client, apiBase, "redis_cache", map[string]interface{}{
		"name":      "bandwidth_throttle",
		"type":      "bandwidth",
		"stream":    "downstream",
		"toxicity":  1.0,
		"auto_heal": 180,
		"attributes": map[string]interface{}{
			"rate": 384,
		},
	})

	addToxic(client, apiBase, "stripe_payment_api", map[string]interface{}{
		"name":      "transient_502_spike",
		"type":      "http_error",
		"stream":    "downstream",
		"toxicity":  0.35,
		"auto_heal": 240,
		"attributes": map[string]interface{}{
			"status": 502,
		},
	})
}

func createProxy(client *http.Client, apiBase string, data map[string]interface{}) {
	b, _ := json.Marshal(data)
	resp, err := client.Post(apiBase+"/proxies", "application/json", bytes.NewReader(b))
	if err == nil {
		resp.Body.Close()
	}
}

func addToxic(client *http.Client, apiBase, proxy string, data map[string]interface{}) {
	b, _ := json.Marshal(data)
	resp, err := client.Post(fmt.Sprintf("%s/proxies/%s/toxics", apiBase, proxy), "application/json", bytes.NewReader(b))
	if err == nil {
		resp.Body.Close()
	}
}

func runTrafficLoop() {
	var wg sync.WaitGroup

	// Postgres worker
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := net.DialTimeout("tcp", "127.0.0.1:5433", 2*time.Second)
			if err == nil {
				// Startup message
				startup := []byte{0, 0, 0, 20, 0, 3, 0, 0, 'u', 's', 'e', 'r', 0, 'a', 'p', 'p', 0, 0}
				conn.Write(startup)
				buf := make([]byte, 1024)
				conn.Read(buf)

				// Send Query
				query := "SELECT id, balance, status FROM accounts WHERE tenant_id = 'prod_882' LIMIT 50;\x00"
				qLen := 4 + len(query)
				qMsg := append([]byte{'Q', 0, 0, byte(qLen >> 8), byte(qLen)}, []byte(query)...)
				conn.Write(qMsg)
				conn.Read(buf)
				conn.Close()
			}
			time.Sleep(120 * time.Millisecond)
		}
	}()

	// Redis worker
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := net.DialTimeout("tcp", "127.0.0.1:6380", 2*time.Second)
			if err == nil {
				conn.Write([]byte("*2\r\n$3\r\nGET\r\n$19\r\nsession:auth:usr_99\r\n"))
				buf := make([]byte, 1024)
				conn.Read(buf)
				conn.Write([]byte("*3\r\n$3\r\nSET\r\n$15\r\nrate_limit:ip_4\r\n$1\r\n1\r\n"))
				conn.Read(buf)
				conn.Close()
			}
			time.Sleep(80 * time.Millisecond)
		}
	}()

	// HTTP worker
	wg.Add(1)
	go func() {
		defer wg.Done()
		client := &http.Client{Timeout: 3 * time.Second}
		for {
			resp, err := client.Get("http://127.0.0.1:8081/v1/charges?customer_id=cus_99342&source=card")
			if err == nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
			time.Sleep(150 * time.Millisecond)
		}
	}()

	wg.Wait()
}

func main() {
	fmt.Println("Starting mock upstream servers...")
	pgL := startPostgresMock("127.0.0.1:15432")
	defer pgL.Close()

	redisL := startRedisMock("127.0.0.1:16379")
	defer redisL.Close()

	httpSrv := startHTTPMock("127.0.0.1:18080")
	defer httpSrv.Close()

	fmt.Println("Waiting for FaultCraft API on http://127.0.0.1:8474...")
	time.Sleep(1 * time.Second)

	setupProxiesAndToxics("http://127.0.0.1:8474")
	fmt.Println("Proxies & Toxics configured. Generating live traffic continuously...")

	runTrafficLoop()
}
