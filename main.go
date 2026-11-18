// Entry point root agar `go build -o server .` (pipeline Jenkins) menghasilkan biner server.
// Logika aplikasi identik dengan cmd/server/main.go.
package main

import (
	"log"

	"app/internal/config"
	"app/internal/server"
)

func main() {
	cfg := config.Load()

	r, err := server.Bootstrap(cfg)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Delcom Blog API berjalan di http://localhost%s (Swagger: /swagger/index.html)", cfg.Address())
	if err := r.Run(cfg.Address()); err != nil {
		log.Fatal(err)
	}
}
