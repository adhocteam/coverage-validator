all: bin/coverage-validator

.PHONY: cross-compile

cross-compile:
	GOOS=linux GOARCH=amd64 go install

bin/coverage-validator: cross-compile
	cp $$GOPATH/$@ $@
