package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.cms.gov/CMS-WDS/marketplace-api/marketplace/core"
	"github.cms.gov/CMS-WDS/marketplace-api/marketplace/coverage"

	log "github.com/Sirupsen/logrus"
	js "github.com/xeipuuv/gojsonschema"
)

var (
	logger    *log.Logger
	npiLookup *coverage.InMemoryNPILookup
	npiFile   = flag.String("d", "npis.csv", "path to NPI file")
	hasHeader = flag.Bool("r", true, "whether NPI file has a CSV header row")
)

func loadNPIs() error {
	file, err := os.Open(*npiFile)
	if err != nil {
		return fmt.Errorf("error opening NPI file: %s", *npiFile)
	}
	defer file.Close()

	npiLookup = coverage.NewInMemoryNPILookup()
	reader := csv.NewReader(file)
	t0 := time.Now()

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading NPI file: %v", err)
		}

		if *hasHeader {
			*hasHeader = false
			continue
		}

		npi, err := strconv.Atoi(record[0])

		if err != nil {
			logger.Infof("error converting NPI string to int: %q", record[0])
			return err
		}

		// some npis in the file do not have types associated with them
		if record[1] != "" {
			entity, err := strconv.Atoi(record[1])
			if err != nil {
				logger.Infof("error converting entity string to int: %q", record[1])
				return err
			}
			npiLookup.NPIProviderType[npi] = entity
		}
	}

	logger.Infof("loaded %d NPIs in %v", len(npiLookup.NPIProviderType), time.Now().Sub(t0))
	return nil
}

func main() {
	flag.Parse()

	logger = &log.Logger{
		Out:       os.Stderr,
		Formatter: &log.TextFormatter{FullTimestamp: true},
		Level:     log.InfoLevel,
	}

	if err := loadNPIs(); err != nil {
		logger.Fatalf("error loading npis: %v", err)
	}

	var (
		plansSchema     = flag.String("plans", "plans_schema.json", "plans JSON schema")
		providersSchema = flag.String("providers", "providers_schema.json", "providers JSON schema")
		drugsSchema     = flag.String("drugs", "drugs_schema.json", "drugs JSON schema")
		indexSchema     = flag.String("index", "index_schema.json", "index JSON schema")
	)

	validator := NewValidator()

	for _, s := range []struct {
		name, filename string
	}{
		{"plans", *plansSchema},
		{"providers", *providersSchema},
		{"drugs", *drugsSchema},
		{"index", *indexSchema},
	} {
		f, err := os.Open(s.filename)
		if err != nil {
			log.Fatalf("opening %s schema from file %s: %v", s.name, s.filename, err)
		}
		if err := validator.Add(s.name, f); err != nil {
			log.Fatalf("adding %s schema from file %s: %v", s.name, s.filename)
		}
		f.Close()
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	http.HandleFunc("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "docs.html")
	})
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.Handle("/validate", validator)
	http.HandleFunc("/schema/", func(w http.ResponseWriter, r *http.Request) {
		schemaName := r.URL.Path[len("/schema/"):]
		validator.ServeFile(schemaName, w)
	})

	port := "8080"
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}
	addr := net.JoinHostPort("0.0.0.0", port)
	done := make(chan struct{})
	go func() {
		logger.Fatal(http.ListenAndServe(addr, nil))
		done <- struct{}{}
	}()
	logger.Infof("validator listening on http://%s/", addr)
	<-done
}

type Validator map[string]*schema

type schema struct {
	parsed   *js.Schema
	contents []byte
}

func NewValidator() Validator {
	return make(Validator)
}

// Add adds a schema by name to the internal registry. Note that it consumes
// the passed-in io.Reader so callers should be aware.
func (v Validator) Add(name string, r io.Reader) error {
	var err error
	s := &schema{}
	s.contents, err = ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	s.parsed, err = js.NewSchema(js.NewStringLoader(string(s.contents)))
	if err != nil {
		return err
	}
	v[name] = s
	return nil
}

var ErrSchemaUnknown = errors.New("validator: unknown schema")

const maxValidationErrs = 500

func (v Validator) Validate(schemaName string, schemaYearFlag int, jsonDoc io.Reader) core.ValidationResult {
	// validation is the same for 2017 and 2018, so use 2017 for 2018
	if schemaYearFlag == coverage.SchemaYear2018 {
		schemaYearFlag = coverage.SchemaYear2017
	}

	switch schemaName {
	case "providers":
		validator := coverage.NewStreamingProviderValidator(jsonDoc, schemaYearFlag, maxValidationErrs)
		return validator.Valid(context.Background(), npiLookup)
	case "drugs":
		validator := coverage.NewStreamingDrugValidator(jsonDoc, schemaYearFlag, maxValidationErrs)
		return validator.Valid(context.Background())
	case "index":
		validator := coverage.NewIndexDocValidator(jsonDoc)
		return validator.Validate(context.Background())
	case "plans":
		validator := coverage.NewStreamingPlanValidator(jsonDoc, schemaYearFlag, maxValidationErrs)
		return validator.Valid()
	default:
		return coverage.NewValidationErrorResult(ErrSchemaUnknown)
	}
}

func (v Validator) ServeFile(schemaName string, w http.ResponseWriter) {
	schema, ok := v[schemaName]
	if !ok {
		http.Error(w, http.StatusText(404), 404)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	if _, err := w.Write(schema.contents); err != nil {
		log.Printf("error writing schema %s contents to HTTP response: %v", schemaName, err)
		http.Error(w, http.StatusText(500), 500)
	}
}

func (v Validator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, http.StatusText(405), 405)
		return
	}
	var resp ValidationResponse
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		resp = multipartFormValidate(v, w, r)
	} else {
		jsonDoc := r.FormValue("json")
		year, err := strconv.Atoi(r.FormValue("schemaYear"))
		if err != nil {
			logger.Errorf("error converting schemaYear %q to int", r.FormValue("schemaYear"))
		}
		resp.SchemaYear = coverage.Year2SchemaYear(year)
		result := v.Validate(r.FormValue("schema"), resp.SchemaYear, bytes.NewBufferString(jsonDoc))
		renderWarningsErrors(w, &resp, &result)
		resp.Schema = r.FormValue("schema")
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, http.StatusText(500), 500)
	}
}

func multipartFormValidate(v Validator, w http.ResponseWriter, r *http.Request) ValidationResponse {
	var resp ValidationResponse
	var result core.ValidationResult
	reader, err := r.MultipartReader()
	if err != nil {
		logger.Errorf("There was an error: %s\n", err)
	}

	// schemaYear now defaults to 2017 (the last time the schema changed)
	resp.SchemaYear = 2017

	for {
		part, err := reader.NextPart()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				logger.Errorf("There was an error: %s\n", err)
			}
		}
		if part.FormName() == "schemaYear" {
			buff, err := ioutil.ReadAll(part)
			if err != nil {
				logger.Errorf("Error reading schema year - %+v\n", err)
			}
			year, err := strconv.Atoi(string(buff))
			if err != nil {
				logger.Errorf("Error converting schema year to int")
			}
			resp.SchemaYear = coverage.Year2SchemaYear(year)
		}
		if part.FormName() == "schema" {
			buff, err := ioutil.ReadAll(part)
			if err != nil {
				logger.Errorf("Error reading schema name - %+v\n", err)
			}
			resp.Schema = string(buff)
		}
		if part.FormName() == "json" {
			result = v.Validate(resp.Schema, resp.SchemaYear, part)
			renderWarningsErrors(w, &resp, &result)
		}
	}
	return resp
}

func renderWarningsErrors(w http.ResponseWriter, resp *ValidationResponse, result *core.ValidationResult) {
	if len(result.Errs) != 0 {
		resp.Valid = false
		if result.Errs[0] == ErrSchemaUnknown {
			resp.Errors = []string{fmt.Sprintf("This schema is unknown: %q", resp.Schema)}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				http.Error(w, http.StatusText(500), 500)
			}

		}
	} else {
		resp.Errors = []string{}
		resp.Valid = true
	}
	for _, err := range result.Errs {
		resp.Errors = append(resp.Errors, err.Error())
	}

	if len(result.Warnings) == 0 {
		resp.Warnings = []string{}
	}
	for _, warning := range result.Warnings {
		resp.Warnings = append(resp.Warnings, warning.Warning())
	}
}

type ValidationResponse struct {
	Valid      bool     `json:"valid"`
	Errors     []string `json:"errors"`
	Warnings   []string `json:"warnings"`
	Schema     string   `json:"schema"`
	SchemaYear int      `json:"year"`
}
