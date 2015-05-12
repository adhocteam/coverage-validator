package main

import (
	"bytes"
	"encoding/json"
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

	http.HandleFunc("/", validator.index)
	http.Handle("/validate", validator)
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
	schemas   map[string]*js.Schema
	templates map[string]*template.Template
}

func NewValidator(plans, providers, drugs string) (*Validator, error) {
	v := new(Validator)
	v.schemas = make(map[string]*js.Schema)
	v.templates = make(map[string]*template.Template)

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

	v.templates["index.html"], err = template.ParseFiles("index.html")
	if err != nil {
		return nil, err
	}

	return v, nil
}

func (v *Validator) index(w http.ResponseWriter, r *http.Request) {
	if err := v.templates["index.html"].Execute(w, struct {
		Example string
	}{
		plansExample,
	}); err != nil {
		log.Printf("rendering html: %v", err)
		http.Error(w, http.StatusText(500), 500)
	}
}

func (v *Validator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, http.StatusText(405), 405)
		return
	}
	var resp ValidationResult
	render := func() {
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, http.StatusText(500), 500)
		}
	}
	jsonDoc := r.FormValue("json")
	if r.FormValue("try") == "1" {
		jsonDoc = plansExample
	}
	resp.PPrint = jsonDoc
	resp.DocType = r.FormValue("doctype")
	schema, ok := v.schemas[r.FormValue("doctype")]
	if !ok {
		resp.Errors = []string{fmt.Sprintf("This document type schema not yet implemented: %q", r.FormValue("doctype"))}
		render()
		return
	}
	loader := js.NewStringLoader(jsonDoc)
	result, err := schema.Validate(loader)
	if err != nil {
		resp.Errors = []string{"JSON is not well-formed: " + err.Error()}
	} else {
		if result.Valid() {
			resp.Valid = true
			pprintJSON(&resp.PPrint)
		} else {
			for _, err := range result.Errors() {
				msg := err.Context.String() + ": " + err.Description
				resp.Errors = append(resp.Errors, msg)
			}
		}
	}
	render()
}

type ValidationResult struct {
	Valid   bool     `json:"valid"`
	Errors  []string `json:"errors"`
	DocType string   `json:"doctype"`
	PPrint  string   `json:"pprint"`
}

func abs(path string) string {
	p, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	return p
}

func pprintJSON(ugly *string) {
	var out bytes.Buffer
	json.Indent(&out, []byte(*ugly), "", "    ")
	*ugly = out.String()
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
