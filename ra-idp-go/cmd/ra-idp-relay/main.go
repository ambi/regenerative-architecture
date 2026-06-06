package main

import (
	"log"

	"ra-idp-go/internal/relay"
)

func main() {
	if err := relay.Run(); err != nil {
		log.Fatal(err)
	}
}
