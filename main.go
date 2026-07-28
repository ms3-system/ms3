package main

import "github.com/naseyro/ms3/api"

func main() {
	// call the API Service and keep listening
	server := api.NewServer()
	server.Serve()
}
