# rootly-go

Go types for Rootly API autogenerated from Swagger/OpenAPI definitions.

## Generating the Code

1. Install [oapi-codegen](https://github.com/deepmap/oapi-codegen) with `go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen@v1.9.0` and make sure it is in your `$PATH`
2. Run `make gen` to generate the file
3. Remove any duplicates from the generated file. This can easily be accomplished in a few minutes with the [golang vscode extension](https://marketplace.visualstudio.com/items?itemName=golang.Go). Unfortunately, this process needs to be done manually to ensure no problems occur.
