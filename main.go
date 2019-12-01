package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/mux"
	"github.com/mikihau/rest-job-worker/handler"
	"github.com/mikihau/rest-job-worker/model"
)

// http server adapted from: https://gist.github.com/enricofoltran/10b4a980cd07cb02836f70a4ab3e72d7
func main() {
	listenAddr := ":8080" // hard coded -- should go to a config file

	var logger = log.New(os.Stdout, "", log.LstdFlags|log.LUTC|log.Lshortfile)
	logger.Printf("Server is starting...")

	router := mux.NewRouter().StrictSlash(true)
	// TODO: provide informative error messages for requests that errored out -- can do an error formatter
	router.HandleFunc("/jobs", handler.VerifyAuth(handler.CreateJob, []model.Role{model.ReadWrite}, logger)).Methods(http.MethodPost)
	router.HandleFunc("/jobs/{jobId}", handler.VerifyAuth(handler.GetJob, []model.Role{model.ReadWrite, model.ReadOnly}, logger)).Methods(http.MethodGet)
	router.HandleFunc("/jobs/{jobId}", handler.VerifyAuth(handler.ChangeJobStatus, []model.Role{model.ReadWrite}, logger)).Methods(http.MethodPut)

	server := &http.Server{
		Addr:         listenAddr,
		Handler:      handler.Logging(logger)(router),
		ErrorLog:     logger,
		ReadTimeout:  5 * time.Second, // should all be configurable too
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	done := make(chan bool)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	go func() {
		<-quit
		logger.Printf("Server is shutting down...")

		// shut down the server first
		ctxServer, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctxServer); err != nil {
			logger.Printf("Failed to gracefully shutdown the server: %v\n", err)
		}

		// then shut down all the Jobs
		ctxJobs, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := handler.TerminateAllJobs(ctxJobs, logger); err != nil {
			logger.Printf("Failed to gracefully shutdown all jobs: %v\n", err)
		}

		close(done)
	}()

	logger.Println("Server is ready to handle requests at", listenAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Could not listen on %s: %v\n", listenAddr, err)
	}

	<-done
	logger.Printf("Server stopped")
}
