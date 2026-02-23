package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/minoru-kitayama/sifto/api/internal/handler"
	"github.com/minoru-kitayama/sifto/api/internal/middleware"
	"github.com/minoru-kitayama/sifto/api/internal/repository"
)

func main() {
	ctx := context.Background()

	db, err := repository.NewPool(ctx)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	sourceRepo := repository.NewSourceRepo(db)
	itemRepo := repository.NewItemRepo(db)
	digestRepo := repository.NewDigestRepo(db)

	sourceH := handler.NewSourceHandler(sourceRepo)
	itemH := handler.NewItemHandler(itemRepo)
	digestH := handler.NewDigestHandler(digestRepo)

	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.Auth)

		r.Route("/sources", func(r chi.Router) {
			r.Get("/", sourceH.List)
			r.Post("/", sourceH.Create)
			r.Patch("/{id}", sourceH.Update)
			r.Delete("/{id}", sourceH.Delete)
		})

		r.Route("/items", func(r chi.Router) {
			r.Get("/", itemH.List)
			r.Get("/{id}", itemH.GetDetail)
		})

		r.Route("/digests", func(r chi.Router) {
			r.Get("/", digestH.List)
			r.Get("/latest", digestH.GetLatest)
			r.Get("/{id}", digestH.GetDetail)
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("api listening on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}
