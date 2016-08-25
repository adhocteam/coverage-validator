package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/CMSgov/marketplace-api/marketplace/adapters"
	"github.com/CMSgov/marketplace-api/marketplace/coverage"
	"github.com/CMSgov/marketplace-api/marketplace/drivers"

	log "github.com/Sirupsen/logrus"
	js "github.com/xeipuuv/gojsonschema"
)

var (
	logger       *log.Logger
	providerRepo adapters.DBProviderRepo
)

func main() {
	dbConnectionInfo := os.Getenv("DATABASE_URL")
	logger = &log.Logger{
		Out:       os.Stderr,
		Formatter: &log.TextFormatter{FullTimestamp: true},
		Level:     log.InfoLevel,
	}

	dbUp := make(chan error)
	const maxDbAttempts = 2
	dbconn := func(conninfo string, done chan<- error) (*drivers.PostgresHandler, error) {
		var dbErr error

		handler, err := drivers.NewPostgresHandler(dbConnectionInfo, logger)
		if err != nil {
			return nil, err
		}

		go func() {
			for i := 0; i < maxDbAttempts; i++ {
				time.Sleep(time.Duration(i) * time.Second)
				if dbErr = handler.TestConnection(); dbErr != nil {
					logger.WithError(dbErr).Error("db connection error")
					continue
				}
				break
			}
			dbUp <- dbErr
		}()

		return handler, nil
	}

	dbHandler, err := dbconn(dbConnectionInfo, dbUp)
	if err != nil {
		panic(err)
	}

	waitDbUp := func(dbUp <-chan error) {
		if err = <-dbUp; err != nil {
			fmt.Printf("err = %+v\n", err)
			logger.WithError(err).Fatalf("couldn't connect to db, giving up")
		}
	}

	waitDbUp(dbUp)

	dbHandlers := make(map[string]adapters.DBHandler)
	dbHandlers["DBProviderRepo"] = dbHandler

	providerRepo = *adapters.NewDBProviderRepo(dbHandlers)

	var (
		plansSchema     = flag.String("plans", "plans_schema.json", "plans JSON schema")
		providersSchema = flag.String("providers", "providers_schema.json", "providers JSON schema")
		drugsSchema     = flag.String("drugs", "drugs_schema.json", "drugs JSON schema")
		indexSchema     = flag.String("index", "index_schema.json", "index JSON schema")
	)

	flag.Parse()

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
		log.Fatal(http.ListenAndServe(addr, nil))
		done <- struct{}{}
	}()
	log.Printf("validator listening on http://%s/", addr)
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

func (v Validator) Validate(schemaName string, jsonDoc io.Reader) error {
	switch schemaName {
	case "providers":
		validator := &coverage.StreamingProviderValidator{
			Dec: json.NewDecoder(jsonDoc),
		}
		return validator.Valid(&providerRepo)

	case "drugs":
		validator := coverage.NewStreamingDrugValidator(jsonDoc)
		return validator.Valid()
	case "index":
		validator := coverage.NewIndexDocValidator(jsonDoc)
		return validator.Validate()
	case "plans":
		validator := coverage.NewStreamingPlanValidator(jsonDoc)
		return validator.Valid()
	default:
		return ErrSchemaUnknown
	}

	return nil
}

func (v Validator) ServeFile(schemaName string, w http.ResponseWriter) {
	schema, ok := v[schemaName]
	if !ok {
		http.Error(w, http.StatusText(404), 404)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	if _, err := w.Write(schema.contents); err != nil {
		log.Printf("error writing schema %s contents to HTTP response: %v", err)
		http.Error(w, http.StatusText(500), 500)
	}
}

func (v Validator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, http.StatusText(405), 405)
		return
	}
	var resp ValidationResult

	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		resp = multipartFormValidate(v, w, r)
	} else {
		jsonDoc := r.FormValue("json")
		err := v.Validate(r.FormValue("schema"), bytes.NewBufferString(jsonDoc))
		if err != nil {
			resp.Valid = false
			renderError(w, &resp, err)
		} else {
			resp.Valid = true
		}
	}

	if resp.Valid {
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, http.StatusText(500), 500)
		}
	}
}

func multipartFormValidate(v Validator, w http.ResponseWriter, r *http.Request) ValidationResult {
	var resp ValidationResult

	reader, err := r.MultipartReader()
	if err != nil {
		logger.Errorf("There was an error: %s\n", err)
	}

	for {
		part, err := reader.NextPart()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				logger.Errorf("There was an error: %s\n", err)
			}
		}

		if part.FormName() == "schema" {
			buff, err := ioutil.ReadAll(part)
			if err != nil {
				logger.Errorf("Error reading schema name - %+v\n", err)
			}
			resp.Schema = string(buff)
		}
		if part.FormName() == "json" {
			err = v.Validate(resp.Schema, part)
			if err != nil {
				renderError(w, &resp, err)
			} else {
				resp.Valid = true
			}
		}
	}
	return resp
}

func renderError(w http.ResponseWriter, resp *ValidationResult, err error) {
	render := func() {
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, http.StatusText(500), 500)
		}
	}

	if err != nil {
		if err == ErrSchemaUnknown {
			resp.Errors = []string{fmt.Sprintf("This schema is unknown: %q", resp.Schema)}
			render()
		}
		resp.Errors = []string{err.Error()}
		render()
	}

}

type ValidationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors"`
	Schema string   `json:"schema"`
}
