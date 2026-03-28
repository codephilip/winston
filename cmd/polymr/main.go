package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/polymr/polymr/internal/notify"
	"github.com/polymr/polymr/internal/router"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := router.New()

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		log.Printf("Polymr router listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			notify.Shutdown("server error: " + err.Error())
			log.Fatalf("server error: %v", err)
		}
	}()

	notify.Startup()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	log.Printf("Shutting down (signal: %v)...", sig)
	notify.Shutdown(sig.String())
}
