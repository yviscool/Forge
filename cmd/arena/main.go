package main

import (
	"github.com/Project-LemonLime/lemonlime-arena/internal/arena"
	"log"
	"net/http"
	"os"
)

func main() {
	addr := os.Getenv("ARENA_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("LemonLime Arena listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, arena.NewServer(arena.NewService())))
}
