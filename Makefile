all: serve

serve:
	go run validator.go plans_schema.json providers_schema.json drugs_schema.json
