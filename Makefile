all: serve

serve:
	godep go run validator.go plans_schema.json providers_schema.json drugs_schema.json
