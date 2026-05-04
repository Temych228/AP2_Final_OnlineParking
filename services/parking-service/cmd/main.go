package main

import (
	"log"

	"parking-service/internal/app"
)

func main() {
	application, err := app.NewApp()
	if err != nil {
		log.Fatal(err)
	}

	application.Run()
}
