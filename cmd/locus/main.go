package main

import (
	"log"

	"github.com/Versifine/locus/internal/config"
)

func main() {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatal("Error loading config:", err)
	}
	log.Printf("Config loaded: %+v\n", cfg)

}
