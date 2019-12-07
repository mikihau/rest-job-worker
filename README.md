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
    - Requires 1 or 2 cases in authentication/authorization layer and networking only.

### Design
- The entire program should be driven by the HTTP server. The following http endpoints are the handles to all possible actions to interact with the service:
    - `POST /jobs`: verifies auth, register a new job
    - `PUT /jobs/<id> {"status": "started"}`: verifies auth, verifies job related status/configs/resources/states, hands the job to workers (update: didn't end up using a queue + worker pool model; the program starts new goroutines for jobs), return information (no callbacks for now)
    - `PUT /jobs/<id> {"status": "stopped"}`: verifies auth, verifies job related states, sends the signal, return information (if soft stop this operation should be async too)
    - `GET /jobs/<id>`: verifies auth, return information
- A job's lifecycle: created, started and stopped. All jobs should be kept in a centralized place ready for lookup and modification. There should be at least 3 reasons a job is stopped:
    - someone asked to stop it via API,
    - timed out because it runs for too long,
    - the service spawning it is shutting down. (Technically we don't have to, but for this program it's best to shut them down because otherwise these processes become orphaned. Multiple service instances with better tracking, or decoupling job runners with the server can be solutions to this.)
- Security
    - assumes authn/z provider (like oauth or jwt tokens)
    - use ssl/tls

### Implementation updates
- WONT DO: DB or just a place for data persistence. For POC I think it's fine without it, but we need it in a real service. Implementation wise, the caveat is that DB access takes time and can fail, so error handling is the key. A practice in Go seems to be creating/extending a `context` when accepting a request, and pass it down the stack, to account for cancelation scenarios.
- WONT DO: configuration. Ideally all/most params (magic numbers in the code) should be configurable. Gophers seems to favor a separate dir containing the configs.
- MOCK: Authn/z -- token based auth is now mocked out, since there's no real token provider.
- DANGER: secrets have been committed to this repo under `ssl/` to show how an https server could work.
- TODO: Write tests for network component.
- TODO: Check for memory leaks -- can be done by installing inspection tools (pprof?) to this app, throw some workload at it, and monitor for any increase in memory related metrics.

### Instructions
- Prerequisites
    - Docker.
    - Go version 1.13 (if you want to run tests).

- Get a build
```
git clone https://github.com/mikihau/rest-job-worker.git
cd rest-job-worker
docker build . -t rest-job-worker
```

- Test (Note: the test suite is a POC so it doesn't have much coverage.)  
In directory `rest-job-worker`, run `go test`.

- Run the program  
`docker run --name rest-job-worker rest-job-worker`. Or if you prefer, publish a host port using option `-p`: `docker run --name rest-job-worker -p <host_port>:443 rest-job-worker`.

### How to: some example scenarios
Now you have a running docker container named "rest-job-worker", as instructed by "Run the program" above. From there, get a bash shell from the running container using `docker exec -it rest-job-worker bash`. The examples below are commands running from within the container shell, but you can modify them accordingly if you prefer to run them elsewhere (mostly sorting out SSL/auth).

- Creating and starting a process, then viewing the logs:
```shell
# create a job
ID=$(curl -X POST https://localhost/jobs -H 'Authorization: writer' -d '{"command": "ls -lah"}' --cert ssl/client.full.pem --cacert ssl/ca.pem | jq --raw-output '.id')

# start the job
curl -X PUT https://localhost/jobs/${ID} -d '{"status":"started"}' -H 'Authorization: writer' --cert ssl/client.full.pem --cacert ssl/ca.pem

# see the status of the job, and log if it's completed
sleep 3 && curl -X GET https://localhost/jobs/${ID} -H 'Authorization: reader' --cert ssl/client.full.pem --cacert ssl/ca.pem
```

- Jobs can time out if running for too long:
```shell
# create a job
ID=$(curl -X POST https://localhost/jobs -H 'Authorization: writer' -d '{"command": "sleep 60"}' --cert ssl/client.full.pem --cacert ssl/ca.pem | jq --raw-output '.id')

# start the job
curl -X PUT https://localhost/jobs/${ID} -d '{"status":"started"}' -H 'Authorization: writer' --cert ssl/client.full.pem --cacert ssl/ca.pem

# see status of the job
sleep 35 && curl -X GET https://localhost/jobs/${ID} -H 'Authorization: reader' --cert ssl/client.full.pem --cacert ssl/ca.pem
# => {"id":"0000","command":"sleep 60","status":"stopped","logs":"","reasonForExit":"timeout"}
```

- Stopping a job:
```shell
# create a job
ID=$(curl -X POST https://localhost/jobs -H 'Authorization: writer' -d '{"command": "sleep 20"}' --cert ssl/client.full.pem --cacert ssl/ca.pem | jq --raw-output '.id')

# start the job
curl -X PUT https://localhost/jobs/${ID} -d '{"status":"started"}' -H 'Authorization: writer' --cert ssl/client.full.pem --cacert ssl/ca.pem

# job should be running ...
curl -X GET https://localhost/jobs/${ID} -H 'Authorization: reader' --cert ssl/client.full.pem --cacert ssl/ca.pem
# => {"id":"0000","command":"sleep 20","status":"started","logs":"","reasonForExit":""}

# stop the job
curl -X PUT https://localhost/jobs/${ID} -d '{"status":"stopped"}' -H 'Authorization: writer' --cert ssl/client.full.pem --cacert ssl/ca.pem

# see status of the job
curl -X GET https://localhost/jobs/${ID} -H 'Authorization: reader' --cert ssl/client.full.pem --cacert ssl/ca.pem
# => {"id":"0000","command":"sleep 20","status":"stopped","logs":"","reasonForExit":"API"}
```

### References
https://gist.github.com/enricofoltran/10b4a980cd07cb02836f70a4ab3e72d7  
https://thenewstack.io/make-a-restful-json-api-go/  
https://gist.github.com/mtigas/952344  
https://dev.to/davidsbond/golang-debugging-memory-leaks-using-pprof-5di8  
