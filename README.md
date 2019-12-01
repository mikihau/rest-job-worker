# rest-job-worker
A prototype job worker service that provides an API to run arbitrary Linux processes. Spec: https://s3-us-west-2.amazonaws.com/elpha.gravitational.io/V3Systems+Engineer.pdf .

### Requirement breakdown / interpretations / assumptions
- Executes arbitrary linux processes -- does it have side effects, meaning it's meant to affect the server/target, or just a pure procedure to be run anywhere?
    - If the purpose is to affect the target, then it needs to be run on the target (will go down this route because we're simulating deployments).
    - If it can be run anywhere, then we can just run it in a container for encapsulation/security.

- Actions: start, stop, query status.
    - Assume stop means emergency stop for now, i.e. SIGKIL.
    - Bonus if i can get to softer ways to stop, SIGTERM/SIGINT etc.
    - Translates to these HTTP REST endpoints: `PUT /job/status {"value": "started"}`, `PUT /job/status {"value": "stopped"}`, `GET /job/status`.
    - The spec seems to solicit a client implementation, but i won't do that for now because it takes time to add a cli based client.

- Get the output of a running job process.
    - Get all outputs at the end (straight forward to implement so do this for now), or 
    - stream outputs while job is running (interesting to look into).
    - To get the logs, `GET /job/status` will have logs if job is finished (for streamed logs maybe look into websockets or something).

- Testing
    - Requires 1 or 2 cases in authentication/authorization layer and networking.

### Design
- Workflow
    - A brief discription of the http endpoints and what they do:
        - `POST /jobs`: verifies auth, register a new job
        - `PUT /jobs/<id> {"status": "started"}`: verifies auth, verifies job related status/configs/resources/states, hands the job to workers (update: didn't end up using a queue + worker pool model; the program starts new goroutines for jobs), return information (no callbacks for now)
        - `PUT /jobs/<id> {"status": "stopped"}`: verifies auth, verifies job related states, sends the signal, return information (if soft stop this operation should be async too)
        - `GET /jobs/<id>`: verifies auth, return information
    - A job's lifecycle: created, started and stopped. There should be at least 3 reasons a job is stopped: someone asked to stop it via API, timed out because it runs for too long, the service spawning it is shutting down.
- Security
    - assumes authn/z provider (like oauth or jwt tokens)
    - use ssl/tls
- Synchronization related (data races, deadlocks etc)
    - would be good to use an automated checker

### Todo
- Tests for network component.
- SSL/TLS: haven't got to setting it up. Thinking of running another sidecar process (nginx maybe) to terminate SSL.
- Authn/z: token based auth is now mocked out -- probably won't get to searching for a real token provider.

### Instructions
- Get a build/package
```
git clone https://github.com/mikihau/rest-job-worker.git
cd rest-job-worker
go build
```

- Test (Note: the test suite is a POC so it doesn't have much coverage.)
`go test`

- Run the program
`go build && ./rest-job-worker`

### How to: some example scenarios
- Creating and starting a process, then viewing the logs:
```shell
# create a job
ID=$(curl -X POST localhost:8080/jobs -H 'Authorization: writer' -d '{"command": "ls -lah"}' | jq --raw-output '.id')

# start the job
curl -X PUT localhost:8080/jobs/${ID} -d '{"status":"started"}' -H 'Authorization: writer'

# see the status of the job, and log if it's completed
sleep 3 && curl -X GET localhost:8080/jobs/${ID} -H 'Authorization: reader'
```

- Jobs can time out if running for too long:
```shell
# create a job
ID=$(curl -X POST localhost:8080/jobs -H 'Authorization: writer' -d '{"command": "sleep 60"}' | jq --raw-output '.id')

# start the job
curl -X PUT localhost:8080/jobs/${ID} -d '{"status":"started"}' -H 'Authorization: writer'

# see status of the job
sleep 35 && curl -X GET localhost:8080/jobs/${ID} -H 'Authorization: reader'
# => {"id":"0000","command":"sleep 60","status":"stopped","logs":"","reasonForExit":"timeout"}
```

- Stopping a job:
```shell
# create a job
ID=$(curl -X POST localhost:8080/jobs -H 'Authorization: writer' -d '{"command": "sleep 30"}' | jq --raw-output '.id')

# start the job
curl -X PUT localhost:8080/jobs/${ID} -d '{"status":"started"}' -H 'Authorization: writer'

# job should be running ...
curl -X GET localhost:8080/jobs/${ID} -H 'Authorization: reader'
# => {"id":"0000","command":"sleep 30","status":"started","logs":"","reasonForExit":""}

# stop the job
curl -X PUT localhost:8080/jobs/${ID} -d '{"status":"stopped"}' -H 'Authorization: writer'

# see status of the job
curl -X GET localhost:8080/jobs/${ID} -H 'Authorization: reader'
# => {"id":"0000","command":"sleep 30","status":"stopped","logs":"","reasonForExit":"API"}
```
