#!/bin/sh
# script for building a linux binary on a mac/windows running go

GOOS=linux GOARCH=amd64 go build -o bin/init-db-amd64-linux init_db.go
GOOS=linux GOARCH=amd64 go build -o bin/beam-amd64-linux beam.go
GOOS=linux GOARCH=amd64 go build -o bin/clean-db-amd64-linux clean_db.go
GOOS=linux GOARCH=amd64 go build -o bin/server-amd64-linux server.go
