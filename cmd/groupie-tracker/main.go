package main

import (
	"log"

	"groupie-tracker/internal/server"
)

func main() {
	if err := server.Run(":8080", 0); err != nil {
		log.Fatal(err)
	}
}
