package main

import (
	"log"

	"github.com/Versifine/locus/internal/config"
	"github.com/Versifine/locus/internal/proxy"
)

func main() {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatal("Error loading config:", err)
	}
	server := proxy.NewServer(
		cfg.Listen.Host+":"+string(cfg.Listen.Port),
		cfg.Backend.Host+":"+string(cfg.Backend.Port),
	)
	err = server.Start()
	if err != nil {
		log.Fatal("Error starting server:", err)
	}

}
