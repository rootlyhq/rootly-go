build:
	go build ./...

test:
	go test ./...

gen:
	cd ./internal/gen/ && go run gen.go
