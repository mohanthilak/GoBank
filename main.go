package main

import (
	"fmt"
	"log"
)

func main() {
	store, err := NewPostgresStore()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", store)

	// server := NewAPIServer("127.0.0.1:8000", store)
	// server.Run()
}
