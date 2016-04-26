package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	js "github.com/xeipuuv/gojsonschema"
)

func main() {
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

func (v Validator) Validate(schemaName string, jsonDoc string) (*js.Result, error) {
	schema, ok := v[schemaName]
	if !ok {
		return nil, ErrSchemaUnknown
	}
	loader := js.NewStringLoader(jsonDoc)
	return schema.parsed.Validate(loader)
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
	render := func() {
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, http.StatusText(500), 500)
		}
	}
	jsonDoc := r.FormValue("json")
	schemaName := r.FormValue("schema")
	resp.Schema = schemaName
	result, err := v.Validate(schemaName, jsonDoc)
	if err != nil {
		if err == ErrSchemaUnknown {
			resp.Errors = []string{fmt.Sprintf("This schema is unknown: %q", r.FormValue("schema"))}
			render()
			return
		}
		resp.Errors = []string{"JSON is not well-formed: " + err.Error()}
	} else {
		if result.Valid() {
			resp.Valid = true
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
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors"`
	Schema string   `json:"schema"`
}

func abs(path string) string {
	p, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	return p
}
