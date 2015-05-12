package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	js "github.com/xeipuuv/gojsonschema"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <plans_schema.json> <providers_schema.json> <drugs_schema.json>\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() != 3 {
		flag.Usage()
		os.Exit(1)
	}

	validator, err := NewValidator(flag.Arg(0), flag.Arg(1), flag.Arg(2))
	if err != nil {
		log.Fatalln("new validator:", err.Error())
	}

	http.Handle("/", validator)
	http.HandleFunc("/schema/", func(w http.ResponseWriter, r *http.Request) {
		var fn string
		switch typ := r.URL.Path[len("/schema/"):]; typ {
		case "plans":
			fn = flag.Arg(0)
		case "providers":
			fn = flag.Arg(1)
		case "drugs":
			fn = flag.Arg(2)
		default:
			http.Error(w, http.StatusText(404), 404)
			return
		}
		http.ServeFile(w, r, fn)
	})
	port := "8080"
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

type Validator struct {
	schemas map[string]*js.Schema
}

func NewValidator(plans, providers, drugs string) (*Validator, error) {
	v := new(Validator)
	v.schemas = make(map[string]*js.Schema)
	var err error
	for _, x := range []struct {
		name     string
		filename string
	}{
		{"plans", plans},
		{"providers", providers},
		{"drugs", drugs},
	} {
		v.schemas[x.name], err = js.NewSchema(js.NewReferenceLoader("file://" + abs(x.filename)))
		if err != nil {
			log.Printf(x.name)
			return nil, err
		}
	}
	return v, nil
}

func (v *Validator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		errors []string
		valid  bool
	)
	jsonDoc := r.FormValue("json")
	if r.FormValue("try") == "1" {
		r.Method = "POST"
		jsonDoc = plansExample
	}
	if r.Method == "POST" {
		// Validate JSON
		schema, ok := v.schemas[r.FormValue("doc-type")]
		if !ok {
			errors = []string{fmt.Sprintf("This document type schema not yet implemented: %q", r.FormValue("doc-type"))}
			goto render
		}
		loader := js.NewStringLoader(jsonDoc)
		result, err := schema.Validate(loader)
		if err != nil {
			errors = []string{
				"JSON is not well-formed: " + err.Error(),
			}
		} else {
			if result.Valid() {
				valid = true
			} else {
				for _, err := range result.Errors() {
					errors = append(errors, err.String())
				}
			}
		}
	}
render:
	t := template.Must(template.New("index").Parse(`
<!DOCTYPE html>
<html lang="en">
    <head>
        <meta charset="utf-8">
        <title>QHP provider and formulary JSON Schema validator</title>
		<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.4/css/bootstrap.min.css">
		<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.4/css/bootstrap-theme.min.css">
		<script src="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.4/js/bootstrap.min.js"></script>
    </head>
    <body>
		<div class="container">
			<h1>QHP provider and formulary JSON Schema validator</h1>
			<p>
				<a href="https://github.com/CMSgov/QHP-provider-formulary-APIs">Click here for more information</a>
				|
				<a href="?try=1&amp;doc-type=plans">Try an example</a>
			</p>
			{{if .Errors}}
				<div class="text-danger">
					<p>This document is <b>not valid</b>:</p>
					<ul>
					{{range .Errors}}
						<li>{{.}}
					{{end}}
					</ul>
				</div>
			{{end}}
			{{if .Valid}}
				<div class="text-success">
					<p>This document is <b>valid</b>.</p>
				</div>
			{{end}}
			{{if eq .Method "POST"}}
			<pre>{{.JSON}}</pre>
			<hr>
			{{end}}
			<form method="post" action="./">
				<div class="form-group">
					<label for="docType">JSON document type</label>
					<select class="form-control" id="docType" name="doc-type" aria-describedby="docTypeHelp">
						<option value="plans" selected>Plans</option>
						<option value="providers">Providers</option>
						<option value="drugs">Drugs</option>
					</select>
					<span id="docTypeHelp" class="help-block">The type of JSON document to be validated.</span>
				</div>
				<div class="form-group">
					<label for="json">JSON</label>
					<textarea class="form-control" rows="5" id="json" name="json" aria-describedby="helpBlock"></textarea>
					<span id="helpBlock" class="help-block">Paste in your JSON here.</span>
				</div>
				<button type="submit" class="btn btn-default">Validate</button>
			</form>
			<hr>
			<footer>
				<p>Dump schemas:
				<ul>
					<li><a href="/schema/plans">Plans</a>
					<li><a href="/schema/providers">Providers</a>
					<li><a href="/schema/drugs">Drugs</a>
				</ul>
			</footer>
		</div>
    </body>
</html>`))
	if err := t.Execute(w, struct {
		Errors []string
		Valid  bool
		Method string
		JSON   string
	}{
		errors,
		valid,
		r.Method,
		jsonDoc,
	}); err != nil {
		log.Printf("rendering html: %v", err)
		http.Error(w, http.StatusText(500), 500)
	}
}

func abs(path string) string {
	p, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	return p
}

const plansExample = `
[
    {
        "plan_id_type": "HIOS-PLAN-ID",
        "plan_id": "12345XX9876543",
        "marketing_name": "Sample Gold Health Plan",
        "summary_url": "http://url/to/summary/benefits/coverage",
        "marketing_url": "http://url/to/health/plan/information",
        "plan_contact": "email@address.com",
        "network": [
            {
                "network_tier": "PREFERRED"
            },
            {
                "network_tier": "NON-PREFERRED"
            }
        ],
        "formulary": [
            {
                "drug_tier": "GENERIC",
                "mail_order": true,
                "cost_sharing": [
                    {
                        "pharmacy_type": "1-MONTH-IN-RETAIL",
                        "copay_amount": 20.0,
                        "copay_opt": "AFTER-DEDUCTIBLE",
                        "coinsurance_rate": 0.10,
                        "coinsurance_opt": "BEFORE-DEDUCTIBLE"
                    },
                    {
                        "pharmacy_type": "1-MONTH-IN-MAIL",
                        "copay_amount": 0.0,
                        "copay_opt": "NO-CHARGE",
                        "coinsurance_rate": 0.20,
                        "coinsurance_opt": null
                    }
                ]
            },
            {
                "drug_tier": "BRAND",
                "mail_order": true,
                "cost_sharing": [
                    {
                        "pharmacy_type": "1-MONTH-IN-RETAIL",
                        "copay_amount": 15.0,
                        "copay_opt": null,
                        "coinsurance_rate": 0.0,
                        "coinsurance_opt": null
                    },
                    {
                        "pharmacy_type": "1-MONTH-IN-MAIL",
                        "copay_amount": 20.0,
                        "copay_opt": "AFTER-DEDUCTIBLE",
                        "coinsurance_rate": 0.10,
                        "coinsurance_opt": "BEFORE-DEDUCTIBLE"
                    }
                ]
            }
        ],
        "last_updated_on": "2015-03-17"
    }
]
`
