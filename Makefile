TARGET_OS=linux
TARGET_ARCH=amd64

all: bin/coverage-validator

.PHONY: cross-compile

cross-compile:
	GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) go install

bin/coverage-validator: cross-compile
	cp $$GOPATH/bin/$(TARGET_OS)_$(TARGET_ARCH)/$(notdir $@) $@
