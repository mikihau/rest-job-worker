# rest-job-worker
- a prototype job worker service that provides an API to run arbitrary Linux processes
- spec: https://s3-us-west-2.amazonaws.com/elpha.gravitational.io/V3Systems+Engineer.pdf 

### requirement breakdown / interpretations / assumptions
- executes arbitrary linux processes -- does it have side effects, meaning it's meant to affect the server/target, or just a pure procedure to be run anywhere
    - if the purpose is to affect the target, then it needs to be run on the target (will go down this route because the Gravity project assumes containerized images)
    - if it can be run anywhere, then we can run it in a container for encapsulation/security

- actions: start, stop, query status
    - assume stop means emergency stop for now, i.e. SIGKIL
    - bonus if i can get to softer ways to stop, SIGTERM/SIGINT etc
    - translates to these HTTP REST endpoints: `PUT /job/status {"value": "started"}`, `PUT /job/status {"value": "stopped"}`, `GET /job/status`
    - the spec seems to solicit a client implementation, but i won't do that for now because it takes time to add a cli based client

- get the output of a running job process
    - get all outputs at the end (straight forward to implement so do this for now), or 
    - stream outputs while job is running (interesting to look into)
    - to get the logs, GET /job/status will have logs if job is finished (for streamed logs maybe look into websockets or something)

- testing
    - 1 or 2 cases in authentication/authorization layer and networking 

### design
- main workflow
    - a brief discription of the http endpoints and what they do
        - `POST /jobs`: verifies auth, register a new job
        - `PUT /jobs/<id> {"status": "started"}`: verifies auth, verifies job related status/configs/resources/states, hands the job to workers, return information (no callbacks for now)
        - `PUT /jobs/<id> {"status": "stopped"}`: verifies auth, verifies job related states, sends the signal, return information (if soft stop this operation should be async too)
        - `GET /jobs/<id>`: verifies auth, return information
- security
    - assumes authn/z provider (like oauth or jwt tokens)
    - use ssl/tls
- synchronization related (data races, deadlocks etc)
    - would be good to use an automated checker

### instructions
- build/package
- test
- run

### checklist
- code formatting
- check for data races
