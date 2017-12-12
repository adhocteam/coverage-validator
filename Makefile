TARGET_OS = linux
TARGET_ARCH = amd64
RELEASE_DIR = /tmp/coverage-validator-release
SOURCES = index.html docs.html index_schema.json providers_schema.json plans_schema.json drugs_schema.json static npis.csv Procfile
NPI_URL:=

all: install

install:
	go install

.PHONY: cross-compile npis.csv

cross-compile:
	GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) go install

release: cross-compile npis.csv
	mkdir -p /tmp/coverage-validator-release/bin
	rsync -av $(GOPATH)/bin/$(TARGET_OS)_$(TARGET_ARCH)/coverage-validator /tmp/coverage-validator-release/bin
	rsync -av $(SOURCES) $(RELEASE_DIR)
	cd $(RELEASE_DIR)
	tar -czf coverage-validator-release.tar.gz -C /tmp coverage-validator-release

npis.csv:
	rm -f npis.csv
	aws s3 cp $(NPI_URL) .
	bzip2 -df npis-latest.dump.bz2
	./tools/npi-csv < npis-latest.dump > $@
