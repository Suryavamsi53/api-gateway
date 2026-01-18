package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

// Simple downstream service for testing the gateway.
func main() {
	http.HandleFunc("/api/users", handleUsers)
	http.HandleFunc("/api/orders", handleOrders)
	http.HandleFunc("/health", handleHealth)

	log.Println("Downstream service listening on :8081")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
	// Simulate processing time
	time.Sleep(50 * time.Millisecond)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"users":[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]}`)
}

func handleOrders(w http.ResponseWriter, r *http.Request) {
	// Simulate slow endpoint
	time.Sleep(200 * time.Millisecond)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"orders":[{"id":101,"status":"pending"}]}`)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"healthy"}`)
}
