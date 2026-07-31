package main

import (
	"log"
	"net/http"

	"github.com/naseyro/ms3/data-service/internal/api"
	"github.com/naseyro/ms3/data-service/internal/backend/localfs"
)

func main() {
	store := localfs.NewLocalStore("/data")

	handler := api.NewHandler(store)

	router := api.NewRouter(handler)

	log.Println("Data Service running on :8081")
	http.ListenAndServe(":8081", router)
}
