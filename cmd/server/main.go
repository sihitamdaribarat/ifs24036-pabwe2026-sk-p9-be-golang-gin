// Command server menjalankan REST API Delcom Blog.
//
//	@title						Delcom Blog API
//	@version					1.0
//	@description				REST API platform publikasi artikel multi-tier dengan AI Blog Generator yang terintegrasi Search Engine.
//	@BasePath					/
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Masukkan token dengan format: Bearer {token}
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
