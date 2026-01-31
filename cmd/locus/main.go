package main

import (
	"fmt"
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
		fmt.Sprintf("%s:%d", cfg.Listen.Host, cfg.Listen.Port),
		fmt.Sprintf("%s:%d", cfg.Backend.Host, cfg.Backend.Port),
	)
	err = server.Start()
	if err != nil {
		log.Fatal("Error starting server:", err)
	}

}
