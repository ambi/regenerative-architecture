package main

import (
	"log"

	"ra-idp-go/internal/bootstrap"
)

func main() {
	if err := bootstrap.Run(); err != nil {
		log.Fatal(err)
	}
}
