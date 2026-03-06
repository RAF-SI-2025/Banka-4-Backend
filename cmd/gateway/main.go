package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"

	handlers "github.com/RAF-SI-2025/Banka-4-Backend/internal/http/handlers"

	healthClient "github.com/RAF-SI-2025/Banka-4-Backend/internal/clients/health"
)

func main() {
	healthClient, _ := healthClient.New(os.Getenv("HEALTH_SERVICE_ADDR"))

	r := mux.NewRouter()

	r.HandleFunc("/health", handlers.HealthHandler(healthClient)).Methods("GET")

	log.Println("gateway listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
