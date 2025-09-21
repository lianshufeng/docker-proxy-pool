#!/bin/bash
go get -u ./...
go mod tidy
go build ./cmd/proxy-pool
