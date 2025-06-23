package main

import (
	"log"
	"time"

	"groupie-tracker/internal/server"
)

func main() {
	if err := server.Run(":8080", 30*time.Minute); err != nil {
		log.Fatal(err)
	}
}
