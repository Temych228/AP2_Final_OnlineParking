package main

import (
	"log"

	"github.com/Temych228/AP2_Final_OnlineParking/services/parking-service/internal/app"
)

func main() {
	if err := app.RunWithSignal(); err != nil {
		log.Fatal(err)
	}
}
