package main

import (
	"creaves/actions"
	"creaves/models"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}

	fmt.Printf("Starting Consolidation Processor on port %s...\n", port)

	// Database is already initialized via models.init()
	// We can use models.DB directly

	// Setup HTTP handlers
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Status endpoint
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service": "consolidation-processor",
			"status":  "running",
			"version": "1.0.0",
		})
	})

	// Process unprocessed events endpoint
	mux.HandleFunc("/process", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		processor := actions.NewEventProcessor(models.DB)
		count, err := processor.ProcessUnprocessedEvents()
		if err != nil {
			http.Error(w, fmt.Sprintf("Processing error: %v", err), 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"processed": count,
			"status":    "success",
		})
	})

	// Process all events (rebuild)
	mux.HandleFunc("/process/all", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		processor := actions.NewEventProcessor(models.DB)
		count, err := processor.ProcessAllEvents()
		if err != nil {
			http.Error(w, fmt.Sprintf("Processing error: %v", err), 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"processed": count,
			"status":    "success",
		})
	})

	// Get statistics
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		processor := actions.NewEventProcessor(models.DB)
		stats, err := processor.GetConsolidatedStats()
		if err != nil {
			http.Error(w, fmt.Sprintf("Stats error: %v", err), 500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})

	// Background processing loop
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			processBatch()
		}
	}()

	fmt.Printf("Consolidation Processor started on http://127.0.0.1:%s\n", port)
	fmt.Println("Endpoints:")
	fmt.Printf("  GET  /         - Service status\n")
	fmt.Printf("  GET  /health   - Health check\n")
	fmt.Printf("  GET  /stats    - Consolidation statistics\n")
	fmt.Printf("  POST /process  - Process unprocessed events\n")
	fmt.Printf("  POST /process/all - Rebuild consolidated view\n")
	fmt.Println("")
	fmt.Println("Background processing: Every 30 seconds")

	if err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%s", port), mux); err != nil {
		log.Fatal(err)
	}
}

func processBatch() {
	// Process a batch of events in the background
	processor := actions.NewEventProcessor(models.DB)
	count, done, err := processor.ProcessEventsBatch(100)
	if err != nil {
		log.Printf("Background processing: Error: %v", err)
		return
	}

	if count > 0 {
		log.Printf("Background processing: Processed %d events (done: %v)", count, done)
	}
}
