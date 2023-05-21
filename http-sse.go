package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
)

type StreamingData struct {
	Message string `json:"message"`
}

func pingTest(c chan float64, URL string) {
	for {
		timeStart := time.Now()

		if _, err := http.Get(URL); err != nil {
			log.Println(err)
			return
		}

		c <- time.Now().Sub(timeStart).Seconds() * 1000
	}
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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
				log.Println(fmt.Sprintf("write tcp %s->%s: write: %f ms", r.Host, r.RemoteAddr, ping))
			}

		}
	})
	// Start server
	log.Fatal(http.ListenAndServe(":8080", nil))
	log.Println("Server started on port 8080.")
}
