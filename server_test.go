package main

import (
	"github.com/mikihau/rest-job-worker/handler"
	"github.com/mikihau/rest-job-worker/model"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func getStuff(w http.ResponseWriter, r *http.Request, l *log.Logger) {
	w.WriteHeader(http.StatusOK)
}

func TestAuth(t *testing.T) {

	cases := []struct {
		headerName, headerValue string
		code                    int
	}{
		{"", "", http.StatusUnauthorized},
		{"Authorization", "wow", http.StatusUnauthorized},
		{"Authorization", "reader", http.StatusForbidden},
		{"Authorization", "writer", http.StatusOK},
	}

	req, err := http.NewRequest("GET", "/stuff", nil)
	if err != nil {
		t.Fatal(err)
	}

	// this test handler requires Role ReadWrite
	authorizedFunc := handler.VerifyAuth(getStuff, []model.Role{model.ReadWrite}, log.New(os.Stdout, "", log.LstdFlags))
	handler := http.HandlerFunc(authorizedFunc)

	for _, c := range cases {
		rr := httptest.NewRecorder()
		req.Header.Set(c.headerName, c.headerValue)
		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != c.code {
			t.Errorf("Wrong status code: with header %v:%v, expecting %v, but got %v",
				c.headerName, c.headerValue, c.code, status)
		}
	}
}
