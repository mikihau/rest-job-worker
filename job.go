package main

import "time"

// A Job represents a linux process for the server to spawn.
type Job struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	Creator   string    `json:"creator"`
	// Code should contain the content of a script starting with a shebang, but would be nice to be able to support binary executables,
	// and a location where the executable/file can be downloaded
	Code string            `json:"code"`
	Argv []string          `json:"argv"`
	Envp map[string]string `json:"envp"`

	Status     string    `json:"status"`
	ModifiedAt time.Time `json:"modifiedAt"`
	ReturnCode int       `json:"returnCode"`
	Logs       string    `json:"logs"`

	// good to have some other fields:
	// EventHistory (a list of events -- got picked up, started running, finished, scheduled to retry etc)
	// RetryPolicy (retry related -- backoff, max retries, under what condition should we retry etc)
	// CallbackPolicy (callback related -- is it needed, where/how should it deliver to etc)
	// Version (if this api, schema, or service is versioned)
}
