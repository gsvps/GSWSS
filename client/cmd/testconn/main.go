package main

import (
	"context"
	"fmt"
	"os"

	"github.com/gswss/gs-protocol/client/internal/transport"
)

func main() {
	server := "http://127.0.0.1:8787/ws"
	password := "change-me"
	useTLS := false
	if len(os.Args) > 1 {
		server = os.Args[1]
	}
	if len(os.Args) > 2 {
		password = os.Args[2]
	}
	if len(os.Args) > 3 && os.Args[3] == "tls" {
		useTLS = true
	}

	err := transport.TestWorker(context.Background(), transport.RelayConfig{
		ServerURL: server,
		Password:  password,
		UseTLS:    useTLS,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("PASS: Worker connection test succeeded")
}
