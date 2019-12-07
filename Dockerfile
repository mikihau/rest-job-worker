FROM golang:alpine

RUN apk update && apk upgrade && apk add bash && apk add curl && apk add jq

RUN mkdir /app 
ADD . /app/
WORKDIR /app 
RUN go build
CMD ["./rest-job-worker"]