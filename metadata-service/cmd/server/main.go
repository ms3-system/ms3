package main

import (
	"log"
	"os"

	"metadata-service/internal/store"
)

func main() {
	store, err := store.Open(store.DefaultDBPath)
	if err != nil {
		log.Fatalf("Error opening store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Printf("Error closing store: %v", err)
		}
	}()

	// Your server logic here
	os.Exit(0)
}
