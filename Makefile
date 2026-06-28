-include .env
export

.PHONY: run test tunnel

tunnel:
	npx smee-client --url $(SMEE_URL) --target http://localhost:8080/webhook

run:
	go run ./cmd/server