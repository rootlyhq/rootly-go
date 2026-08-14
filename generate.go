//go:generate go run -modfile=./tools/go.mod github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=config.yaml .vendored/rootly-api.json
//go:generate go run -modfile=./tools/go.mod github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=config-spec.yaml .vendored/rootly-api.json
package rootly
