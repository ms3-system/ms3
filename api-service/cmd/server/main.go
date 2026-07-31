package main

import (
	"log"

	"github.com/naseyro/ms3/api-service/api"
	"github.com/naseyro/ms3/api-service/clients/data"
	"github.com/naseyro/ms3/api-service/clients/metadata"
)

const (
	metadataURL = "http://metadata-service:8082"
	dataURL     = "http://data-service:8081"
)

func main() {
	metadataClient := metadata.NewHTTPMetadataClient(metadataURL)
	dataClient := data.NewHTTPDataClient(dataURL)

	srv := api.NewServer(metadataClient, dataClient)

	log.Println("api-service listening on localhost:8080")
	log.Fatal(srv.Run())
}
