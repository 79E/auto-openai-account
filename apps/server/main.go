package main

import (
	"log"
	"net/http"
	"os"

	"github.com/79E/auto-openai-account/internal/api"
	"github.com/79E/auto-openai-account/internal/runner"
	"github.com/79E/auto-openai-account/internal/storage"
)

func main() {
	store, err := storage.Open("data/register.db")
	if err != nil {
		log.Fatalf("open storage: %v", err)
	}
	defer store.Close()

	server := api.New(store, runner.New(store))
	addr := os.Getenv("AUTO_OPENAI_ACCOUNT_LISTEN")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("auto-openai-account listening on %s", addr)
	if err := http.ListenAndServe(addr, server.Routes()); err != nil {
		log.Fatal(err)
	}
}
