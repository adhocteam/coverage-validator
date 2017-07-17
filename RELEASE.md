Deploying a release of the coverage validator
=============================================

This repository is the source of the validator. What gets deployed to the
production Heroku app server is a build artifact, which is versioned in its own
repository][1].

Updating NPI data
------------------

To update the NPI data, download the latest raw NPI data from the NPPES site
which is hosted at CMS and is [freely available to download][npi_files].
Extract the ZIP archive in this directory locally. Then, invoke `make` to
generate the updated CSV file:

``` shell
$ RAWNPPESCSV=name-of-nppes.csv RELEASE_REPO=path-to-release-repo make npis.csv
```

Making the release
------------------

Then create a release which will copy all the data and source files the
validator needs including the NPI CSV, and the updated validator executable to
the release artifact repository, and which point you should commit the changes
to that repo and push to Github to trigger a Heroku build:

``` shell
$ make release
```

[1]: https://github.com/adhocteam/coverage-validator-release
[npi_files]: http://download.cms.gov/nppes/NPI_Files.html
