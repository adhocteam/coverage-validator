TARGET_OS = linux
TARGET_ARCH = amd64
RELEASE_REPO = $(HOME)/work/coverage-validator-release

SOURCES = index.html index_schema.json providers_schema.json plans_schema.json drugs_schema.json static npis.csv Procfile

all: install

install:
	go install

.PHONY: cross-compile

cross-compile:
	GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) go install

release: cross-compile
	mkdir -p $(RELEASE_REPO)/bin
	rsync -av $$GOPATH/bin/$(TARGET_OS)_$(TARGET_ARCH)/coverage-validator $(RELEASE_REPO)/bin/coverage-validator
	rsync -av $(SOURCES) $(RELEASE_REPO)
