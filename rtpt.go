package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type StreamingData struct {
	Message string `json:"message"`
}

var MAX_CONNECTIONS_PER_IP int = 10

type ConnectionPool struct {
	mu          sync.Mutex
	connections map[string]int
}

func (cp *ConnectionPool) GetConnectionLimit() int {
	return MAX_CONNECTIONS_PER_IP
}

func (cp *ConnectionPool) GetRemainingConnections(ip string) int {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if count, ok := cp.connections[ip]; ok {
		return MAX_CONNECTIONS_PER_IP - count
	}

	return MAX_CONNECTIONS_PER_IP
}

func (cp *ConnectionPool) AddConnection(ip string) bool {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if count, ok := cp.connections[ip]; ok && count >= MAX_CONNECTIONS_PER_IP {
		return false
	}

	cp.connections[ip]++
	return true
}

func (cp *ConnectionPool) RemoveConnection(ip string) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if count, ok := cp.connections[ip]; ok && count > 0 {
		cp.connections[ip]--
	}
}

func pingTest(c chan float64, URL string) {
	for {
		timeStart := time.Now()

		if _, err := http.Get(URL); err != nil {
			log.Println(err)
			return
		}

		c <- float64(time.Since(timeStart).Milliseconds())
	}
}

func main() {
	// Create a connection pool
	connectionPool := &ConnectionPool{
		connections: make(map[string]int),
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Check if maximum connections per IP reached
		ip := strings.Split(r.RemoteAddr, ":")[0]
		if !connectionPool.AddConnection(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{
				"message": "Maximum connections per IP reached.",
			})
			return
		}

		// Remove connection from pool when client disconnects
		defer connectionPool.RemoveConnection(ip)

		// Check if request method is GET
		if r.Method != "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]string{
				"message": "Method not allowed.",
			})
			return
		}

		// Get URL parameter
		URL := r.URL.Query().Get("url")
		if len(URL) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"message": "Missing URL parameter.",
			})
			return
		}

		// Check if URL is valid
		if _, err := url.ParseRequestURI(URL); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"message": "Invalid URL parameter.",
			})
			return
		}

		// Check if URL is reachable
		if _, err := http.Get(URL); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"message": "URL is unreachable.",
			})
			return
		}

		// Set headers for connection limit information
		w.Header().Set("X-ConnectionLimit-Limit", fmt.Sprintf("%d", connectionPool.GetConnectionLimit()))
		w.Header().Set("X-ConnectionLimit-Remaining", fmt.Sprintf("%d", connectionPool.GetRemainingConnections(ip)))

		// Create channel
		c := make(chan float64)
		// Start ping test
		go pingTest(c, URL)

		// Set the headers for Server-Sent Events
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Flush the response buffer to initiate the SSE connection
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		// Listen for ping data
		for ping := range c {
			// Send ping data to client as JSON if no error occurs
			if jsonData, err := json.Marshal(StreamingData{Message: fmt.Sprintf("%f ms", ping)}); err != nil {
				log.Println(err)
				return
			} else {
				if _, err = w.Write([]byte(fmt.Sprintf("data: %s\n\n", jsonData))); err != nil {
					log.Println(err)
					return
				}
			}

			// Flush the response buffer after each write
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
				log.Printf("write tcp %s->%s: write: %f ms", r.Host, r.RemoteAddr, ping)
			}

		}
	})
	// Start server
	log.Fatal(http.ListenAndServe(":8080", nil))
	log.Println("Server started on port 8080.")
}
