# qhp-validator

**[Try out the validator](https://qhp-validator.herokuapp.com/)**

[This repository](https://github.com/CMSgov/QHP-provider-formulary-APIs) defines three top-level specifications for publishing qualified health plan (QHP), provider, and drug information from issuing insurance companies for use on the Federal Facilitated Marketplace (FFM), aka [HealthCare.gov](https://healthcare.gov/).

This tool validates JSON documents against schemas defined for each of these three specifications.

It offers two interfaces:

* A web form for pasting JSON content and getting a validation report, and
* A web service for POST'ing JSON content (to `/validate`, see [docs](https://qhp-validator.herokuapp.com/) for details) for automated or programmatic validation.
