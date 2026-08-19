package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
)

func main() {
	if len(os.Args) != 3 {
		fail("usage: install.go VERSION INSTALL_DIRECTORY")
	}

	want, err := version.NewVersion(os.Args[1])
	if err != nil {
		fail("invalid Terraform version: %v", err)
	}

	installer := releases.ExactVersion{
		Product:    product.Terraform,
		Version:    want,
		InstallDir: os.Args[2],
	}
	path, err := installer.Install(context.Background())
	if err != nil {
		fail("unable to verify and install Terraform: %v", err)
	}

	got, err := product.Terraform.GetVersion(context.Background(), path)
	if err != nil {
		fail("unable to identify installed Terraform version: %v", err)
	}
	if !got.Equal(want) {
		fail("installed Terraform version %s does not match pinned version %s", got, want)
	}

	fmt.Printf("Installed verified Terraform %s at %s\n", got, path)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
