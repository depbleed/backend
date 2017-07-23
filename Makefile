.PHONY: all build test

all: build test

heroku:
	go get -u github.com/golang/dep/cmd/dep
	dep ensure -update
	make build

build:
	go build -o bin/depbleed ./depbleed

test:
	go test ./depbleed -covermode=atomic -coverprofile=depbleed.cover.out
	go test ./git -covermode=atomic -coverprofile=git.cover.out
	go test ./persistence -covermode=atomic -coverprofile=persistence.cover.out
	bash -c 'ls *.cover.out | while read file; do go tool cover -func=$${file}; done'
	bash -c 'cat *.cover.out > coverage.txt'
	bash -c 'rm *.cover.out'
