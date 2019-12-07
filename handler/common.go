// Package handler contains handlers and middleware used by the rest job worker.
package handler

import (
	"log"
	"net/http"
	"strings"

	"github.com/mikihau/rest-job-worker/model"
)

// Logging adds logging capabilities to incoming http requests.
func Logging(logger *log.Logger) func(http.Handler) http.Handler {
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

// VerifyAuth verifies that incoming requests are authenticated and authorized.
// It rejects requests with no or improper auth information by responding to requests with HTTP error codes.
// This middleware also injects a logger for handlers to log into just for convenience -- but it's better to take it out separately.
func VerifyAuth(handle func(http.ResponseWriter, *http.Request, *log.Logger), requiredRoles []model.Role, logger *log.Logger) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// Should check with auth provider here, but we mock by populating our "db" with 2 users with read-only and read-write permissions.
		// Can use a db! Like https://github.com/boltdb/bolt.
		users := model.Users{
			Users: []model.User{
				model.User{Name: "reader", Permission: []model.Role{model.ReadOnly}},
				model.User{Name: "writer", Permission: []model.Role{model.ReadWrite}},
			},
		}
		logger.Printf("Checking headers for auth ...\n")
		if authHeader := strings.TrimSpace(r.Header.Get("Authorization")); authHeader == "" {
			logger.Printf("No Authorization header found.\n")
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			// DANGER: need to mock out the auth scheme, for now assume it's just the username -- do not use this in production
			if user, found := users.FindUserByName(authHeader); !found {
				logger.Printf("Not authorized: User %s not found.\n", authHeader)
				w.WriteHeader(http.StatusUnauthorized)
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
