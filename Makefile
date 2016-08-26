TARGET_OS=linux
TARGET_ARCH=amd64

all: bin/coverage-validator

.PHONY: cross-compile

cross-compile:
	GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) go install

bin/coverage-validator: cross-compile
	cp $$GOPATH/bin/$(TARGET_OS)_$(TARGET_ARCH)/$(notdir $@) $@

dropbox:
	mkdir -p ~/Dropbox/coverage-validator-beta/bin
	rsync -av $$GOPATH/bin/$(TARGET_OS)_$(TARGET_ARCH)/coverage-validator ~/Dropbox/coverage-validator-beta/bin/coverage-validator
	rsync -av index.html index_schema.json providers_schema.json plans_schema.json drugs_schema.json static npis.csv Procfile \
		~/Dropbox/coverage-validator-beta
