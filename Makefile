build:
	go build ./...

test:
	go test ./...

fetch-spec:
	curl -fs https://rootly-heroku.s3.amazonaws.com/swagger/v1/swagger.json -o swagger.json

gen:
	go generate ./...
