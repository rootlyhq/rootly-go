// Generate the scheme.go file in the root of this repo

package main

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
)

const binaryName = "oapi-codegen"

func main() {
	codegenPath, err := exec.LookPath(binaryName)
	if err != nil {
		log.Fatal("Is ", binaryName, " installed? ", "\n\n", err)
	}

	cmd := exec.Command(codegenPath, "swagger.json")
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		log.Fatal("Failed to run code gen", "\n\n", err)
	}
	output = []byte(strings.Replace(string(output), "package Swagger", "package rootly", 1))

	err = ioutil.WriteFile("../../schema.gen.go", []byte(output), 0666)
	if err != nil {
		log.Fatal("Failed to write data to scheme file", "\n\n", err)
	}

}
