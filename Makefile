TARGET_OS = linux
TARGET_ARCH = amd64
RELEASE_DIR = /tmp/coverage-validator-release
SOURCES = index.html docs.html index_schema.json providers_schema.json plans_schema.json drugs_schema.json static npis.csv Procfile
NPI_URL = $(npiURL)

all: install

install:
	go install

.PHONY: cross-compile npis.csv

cross-compile:
	GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) go install

release: cross-compile npis.csv
	mkdir -p /tmp/coverage-validator-release/bin
	rsync -av $(GOPATH)/bin/coverage-validator /tmp/coverage-validator-release/bin
	rsync -av $(SOURCES) $(RELEASE_DIR)
	cd $(RELEASE_DIR)
	tar -czf coverage-validator-release.tar.gz -C /tmp coverage-validator-release

npis.csv:
	rm -f npis.csv
	aws s3 cp $(NPI_URL) npis-latest.csv.bz2
	bzip2 -df npis-latest.csv.bz2
	./tools/npi-csv < npis-latest.csv> $@
