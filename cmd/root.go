package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/plural-labs/sonic-relayer/relayer"
)

func Execute() {
	if len(os.Args) != 2 {
		log.Fatalf("expect 1 argument to config file got %d", len(os.Args)-1)
	}

	filePath := os.Args[1]
	config, err := relayer.LoadConfig(filePath)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	err = relayer.Relay(ctx, &config)
	if err != nil {
		log.Fatal(err)
	}
}
