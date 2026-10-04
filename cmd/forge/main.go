package main

import (
	"log"
	"net/http"
	"os"

	"github.com/yviscool/forge/internal/arena"
)

func main() {
	addr := os.Getenv("FORGE_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("==================================================")
	log.Printf("  Forge Contest Host & Evaluation Workbench")
	log.Printf("==================================================")
	log.Printf("Listening on %s", addr)
	log.Printf("Student Lobby:    http://localhost%s/", addr)
	log.Printf("Teacher Console:  http://localhost%s/teacher", addr)
	log.Printf("Real-time Events: http://localhost%s/api/events", addr)
	log.Printf("==================================================")
	log.Fatal(http.ListenAndServe(addr, arena.NewServer(arena.NewService())))
}
