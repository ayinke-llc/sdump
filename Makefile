tidy_dependencies:
	go mod tidy

migrate:
	migrate -path ./datastore/sql/migrations/ -database "postgres://sdump:sdump@localhost:3432/sdump?sslmode=disable" up

migrate-down:
	migrate -path ./datastore/sql/migrations/ -database "postgres://sdump:sdump@localhost:3432/sdump?sslmode=disable" down

run-http:
	go run cmd/*.go

