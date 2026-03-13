package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
}

func main() {
	serviceName := "observability"
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(healthResponse{
			Status:  "ok",
			Service: serviceName,
			Version: "0.1.0",
		})
	})

	log.Println(serviceName + " listening on :" + port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("server failed: " + err.Error())
	}
}
