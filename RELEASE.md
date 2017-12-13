Deploying a release of the coverage validator
=============================================


Updating NPI data
------------------

To update the NPI data, run the npis.csv make target.  This will download the NPIs from
the S3 bucket and will parse and store as a csv file.

``` shell
$ make npis.csv
```

Deploying
------------------

When ready to deploy, run the release target which will update the npis and package the source files into
a tarball to be used in the deploy to heroku.  The deploy-coverage-validator cloudbees jenkins job will call
the release target, upload the tar to heroku, then trigger a deploy.

``` shell
$ make release
```
