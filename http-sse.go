package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type StreamingData struct {
	Message string `json:"message"`
}

func main() {
	ping := make(chan float64)

	go func() {
		for {
			var timeStart, timeEnd time.Time
			timeStart = time.Now()

			_, err := http.Get("https://kuliah.uajy.ac.id")
			if err != nil {
				log.Fatal(err)
			}

			timeEnd = time.Now()
			ping <- timeEnd.Sub(timeStart).Seconds() * 1000
		}
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Set the headers for Server-Sent Events
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Flush the response buffer to initiate the SSE connection
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		for ping := range ping {
			data := StreamingData{
				Message: fmt.Sprintf("%f ms", ping),
			}

			jsonData, err := json.Marshal(data)
			if err != nil {
				log.Println(err)
				return
			}

			event := fmt.Sprintf("data: %s\n\n", jsonData)
			_, err = w.Write([]byte(event))
			if err != nil {
				log.Println(err)
				return
			}

			// Flush the response buffer after each write
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
				log.Println(r.RemoteAddr, data.Message)
			}

		}
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
