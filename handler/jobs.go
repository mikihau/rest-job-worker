package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/mikihau/rest-job-worker/model"
)

// allJobs is a global variable to keep track of jobs. This is because we need something in place of a DB.
// WARNING: jobs WILL BE LOST after this program terminates.
var allJobs = make(map[string]*model.Job)

// jobsWg helps waiting for processes (associated with jobs) to terminate.
var jobsWg = &sync.WaitGroup{}

// CreateJob creates a new Job without starting it.
func CreateJob(w http.ResponseWriter, r *http.Request, l *log.Logger) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		l.Printf("An error occurred while reading response body: %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var job model.Job
	if err := json.Unmarshal(body, &job); err != nil {
		l.Printf("An error occurred while unmarshaling input: %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if job.Command == "" {
		l.Printf("Expecting a command, but not provided.\n")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// just assume arg[0] is the executable for now since we're talking about just a single process
	executable := strings.Fields(job.Command)[0]
	if _, err := exec.LookPath(executable); err != nil {
		l.Printf("Executable not found: %v\n", executable)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	l.Printf("Creating a job with command: %s\n", job.Command)
	// can have a NewJob() function for initialization
	job.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	job.Status = "created"
	job.ManualStop = make(chan string, 1)
	allJobs[job.ID] = &job
	if marshalled, err := json.Marshal(job); err != nil {
		l.Printf("An error occurred while preparing output: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		l.Printf("Created job: %+v\n", job)
		w.Header().Set("Location", r.Host+"/jobs/"+job.ID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write(marshalled)
	}
}

// GetJob returns a job matching the JobID, if it exists.
func GetJob(w http.ResponseWriter, r *http.Request, l *log.Logger) {
	// jobID should be in the vars, otherwise the router wouldn't route to this function
	jobID := mux.Vars(r)["jobId"]
	job, ok := allJobs[jobID]
	if !ok {
		l.Printf("JobID not found: %v\n", jobID)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if marshalled, err := json.Marshal(job); err != nil {
		l.Printf("An error occurred while preparing output: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		l.Printf("Returning job: %v\n", job.ID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(marshalled)
	}
}

// ChangeJobStatus changes the status of the job, currently by starting or stopping the job matching the JobID.
func ChangeJobStatus(w http.ResponseWriter, r *http.Request, l *log.Logger) {
	// can use a schema validator, but for now we just care about the "status" field
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		l.Printf("An error occurred while reading response body: %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var inputJob model.Job
	if err := json.Unmarshal(body, &inputJob); err != nil {
		l.Printf("An error occurred while unmarshaling input: %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if inputJob.Status != "started" && inputJob.Status != "stopped" {
		l.Printf("Expecting status field as 'started' or 'stopped', but got: %v\n", inputJob.Status)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// jobID should be in the vars, otherwise the router wouldn't route to this function
	jobID := mux.Vars(r)["jobId"]
	job, ok := allJobs[jobID]
	if !ok {
		l.Printf("JobID not found: %v\n", jobID)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if inputJob.Status == "started" { // asking to start the job
		if job.Status != "created" {
			l.Printf("Job %v is unstartable because its status is %v\n", job.ID, job.Status)
			w.WriteHeader(http.StatusUnprocessableEntity) //...422?
			return
		}
		jobsWg.Add(1)
		go runJob(job, l) // what happens when the http server gets terminated?
		job.Status = "started"
	} else { // asking to stop the job
		if job.Status != "started" {
			l.Printf("Job %v is unstoppable because its status is %v\n", job.ID, job.Status)
			w.WriteHeader(http.StatusUnprocessableEntity) //...422?
			return
		}
		job.ManualStop <- "API"
		job.Status = "stopped"
	}

	if marshalled, err := json.Marshal(job); err != nil {
		l.Printf("An error occurred while preparing output: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		l.Printf("Job %v status changed to: %v\n", job.ID, job.Status)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(marshalled)
	}
}

// TerminateAllJobs terminates all running processes associated with Jobs by killing each of them. It blocks
// until each of the jobs is killed, or a timeout has reached, whichever happens first.
func TerminateAllJobs(ctx context.Context, l *log.Logger) error {
	for _, job := range allJobs {
		if job.Status == "started" {
			job.ManualStop <- "Service Shutdown"
		}
	}

	channel := make(chan bool)
	go func() {
		defer close(channel)
		jobsWg.Wait() // um what if this never returns?
	}()

	select {
	case <-channel:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func runJob(j *model.Job, l *log.Logger) {
	defer jobsWg.Done()

	// start the process
	cmd := exec.Command("bash", "-c", j.Command) // make sure bash is installed ...
	cmd.Stdout = &j.Output
	cmd.Stderr = &j.Output
	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select { // 3 ways to end the spawned process
	// 1. timeout
	case <-time.After(30 * time.Second): // timeout should be configurable, through a config file and provided as a param on a per Job basis
		if err := cmd.Process.Kill(); err != nil {
			l.Printf("Timeout reached, but failed to kill process of Job %v: %v\n", j.ID, err)
		}
		l.Printf("Process killed for Job %v as timeout reached at %v\n", j.ID, time.Now())
		j.ReasonForExit = "timeout"
	// 2. stopped manually, either by API call or by service shutdown
	case reason := <-j.ManualStop:
		if err := cmd.Process.Kill(); err != nil {
			l.Printf("Stop job request received, but failed to kill process of Job %v: %v\n", j.ID, err)
		}
		l.Printf("Process killed for Job %v, as stop request (via %v) processed, at %v\n", j.ID, reason, time.Now())
		j.ReasonForExit = reason
	// 3. finished by itself (note: if timed out or stopped manually, cmd.Wait() still returns, but the done channel is nonblocking so it's not leaky)
	case err := <-done:
		if err != nil {
			l.Printf("Process finished for Job %v, with error = %v\n", j.ID, err)
			j.ReasonForExit = fmt.Sprintf("process finished with error: %v", err)
		} else {
			l.Printf("Process finished successfully for Job %v\n", j.ID)
			j.ReasonForExit = "process finished"
		}
	}

	j.Logs = string(j.Output.Bytes()) // can have some more escaping, but good for now
	j.Status = "stopped"
}
