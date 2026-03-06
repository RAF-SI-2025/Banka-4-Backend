package handlers

import (
	"encoding/json"
	"net/http"

	pb "github.com/RAF-SI-2025/Banka-4-Backend/proto/health"
)

func HealthHandler(client pb.HealthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := client.Check(r.Context(), &pb.HealthRequest{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		response := map[string]string{
			"health": resp.Status,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
