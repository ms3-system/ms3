package main

import "github.com/naseyro/minis3/api"

func main() {
	// call the API Service and keep listening
	server := api.NewServer()
	server.Serve()
}
