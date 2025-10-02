package main

import (
	"log"
	"time"
)

func main() {
	log.Println("Worker started (placeholder). Polling every 1s…")
	for {
		// TODO: claim job from DB, call provider, save assets, update status.
		time.Sleep(1 * time.Second)
	}
}
