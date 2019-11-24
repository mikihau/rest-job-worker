package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// http server adapted from: https://gist.github.com/enricofoltran/10b4a980cd07cb02836f70a4ab3e72d7

func main() {
	listenAddr := ":8080" // hard coded -- should go to a config file

	logger := log.New(os.Stdout, "", log.LstdFlags|log.LUTC|log.Lshortfile)
	logger.Printf("Server is starting...")

	router := mux.NewRouter().StrictSlash(true)
	// TODO: provide informative error messages for requests that errored out -- can do an error formatter?
	router.HandleFunc("/jobs", verifyAuth(createJob, []Role{ReadWrite}, logger)).Methods(http.MethodPost)
	router.HandleFunc("/jobs/{jobId}", verifyAuth(getJob, []Role{ReadWrite, ReadOnly}, logger)).Methods(http.MethodGet)
	router.HandleFunc("/jobs/{jobId}", verifyAuth(changeJobStatus, []Role{ReadWrite}, logger)).Methods(http.MethodPut)

	server := &http.Server{
		Addr:         listenAddr,
		Handler:      logging(logger)(router),
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

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		server.SetKeepAlivesEnabled(false)
		if err := server.Shutdown(ctx); err != nil {
			logger.Fatalf("Could not gracefully shutdown the server: %v\n", err)
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

func verifyAuth(handle func(http.ResponseWriter, *http.Request, *log.Logger), requiredRoles []Role, logger *log.Logger) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Should check with auth provider here, but we mock by populating our "db" with 2 users with read-only and read-write permissions.
		// TODO: put this in something like a populate function, or use https://github.com/boltdb/bolt
		users := Users{
			[]User{
				User{"reader", []Role{ReadOnly}},
				User{"writer", []Role{ReadWrite}},
			},
		}
		logger.Printf("Checking headers for auth ...\n")
		if authHeader := strings.TrimSpace(r.Header.Get("Authorization")); authHeader == "" {
			logger.Printf("No Authorization header found.\n")
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			// TODO: need to mock out the auth scheme, for now assume it's just the username -- please don't do this in production
			if user, found := users.FindUserByName(authHeader); !found {
				logger.Printf("Not authorized: User %s not found.\n", authHeader)
				w.WriteHeader(http.StatusForbidden)
			} else if !user.HasRequiredRoles(requiredRoles) {
				logger.Printf("Not authorized: roles %v required, but user %s has roles %v\n", requiredRoles, user.Name, user.Permission)
				w.WriteHeader(http.StatusForbidden)
			} else {
				logger.Printf("Requiring roles %v, User %s has roles %v, and is authorized ...\n", requiredRoles, user.Name, user.Permission)
				handle(w, r, logger)
			}
		}
	}
}

func createJob(w http.ResponseWriter, r *http.Request, l *log.Logger) {
	fmt.Fprintln(w, r.Method, r.URL.Path)
}

func getJob(w http.ResponseWriter, r *http.Request, l *log.Logger) {
	fmt.Fprintln(w, r.Method, r.URL.Path)
}

func changeJobStatus(w http.ResponseWriter, r *http.Request, l *log.Logger) {
	fmt.Fprintln(w, r.Method, r.URL.Path)
	//w.Header().Set("Content-Type", "application/json")
	//w.Header().Set("X-Content-Type-Options", "nosniff")
	//w.WriteHeader(http.StatusOK)
}

func logging(logger *log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Println("Incoming request:", r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent())
			defer func() {
				logger.Println("Response returned:", r.Method, r.URL.Path, r.RemoteAddr, r.UserAgent()) // # TODO: figure out a way to log the status code
			}()
			next.ServeHTTP(w, r)
		})
	}
}
