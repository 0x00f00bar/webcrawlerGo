SHELL=/bin/bash
include .env
# export $(shell sed 's/=.*//' .env)
run:
	@echo "Executing go run ..."
	@go run cmd/* -baseurl=$(BASEURL) -db-dsn="$(DB_DSN)"

migrate-up:
	@echo "Migrating up ..."
	@migrate -path "./models/psql/migrations" -database "$(DB_DSN)" up

migrate-down:
	@echo "Migrating down ..."
	@migrate -path "./models/psql/migrations" -database "$(DB_DSN)" down

migrate-create:
	@if [[ "$(MIGRATION_NAME)" == "" ]]; then \
		echo "MIGRATION_NAME argument is required"; \
	else \
		echo "Creating migration ... $(MIGRATION_NAME)"; \
		migrate create -seq -ext ".sql" -dir "./models/psql/migrations" $(MIGRATION_NAME); \
	fi 
