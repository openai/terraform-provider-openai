package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

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

	path, err := installTerraform(context.Background(), want, os.Args[2], "https://releases.hashicorp.com")
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

func installTerraform(ctx context.Context, want *version.Version, directory, releaseURL string) (string, error) {
	upstream, err := url.Parse(releaseURL)
	if err != nil || upstream.Host == "" || (upstream.Scheme != "http" && upstream.Scheme != "https") {
		return "", fmt.Errorf("invalid Terraform release URL: %q", releaseURL)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("listen for verified Terraform releases: %w", err)
	}

	server := &http.Server{
		Handler:           pinnedReleaseProxy(want.String(), upstream),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go server.Serve(listener)
	defer server.Close()

	installer := releases.ExactVersion{
		Product:    product.Terraform,
		Version:    want,
		InstallDir: directory,
		ApiBaseURL: "http://" + listener.Addr().String(),
	}
	return installer.Install(ctx)
}

func pinnedReleaseProxy(want string, upstream *url.URL) http.Handler {
	prefix := "/terraform/" + want + "/"
	checksums := "terraform_" + want + "_SHA256SUMS"
	archive := "terraform_" + want + "_" + runtime.GOOS + "_" + runtime.GOARCH + ".zip"
	index := prefix + "index.json"

	proxy := httputil.NewSingleHostReverseProxy(upstream)
	director := proxy.Director
	proxy.Director = func(request *http.Request) {
		director(request)
		request.Host = upstream.Host
		request.Header.Set("Accept-Encoding", "identity")
	}
	proxy.ModifyResponse = func(response *http.Response) error {
		if response.StatusCode >= http.StatusMultipleChoices && response.StatusCode < http.StatusBadRequest {
			return fmt.Errorf("unexpected Terraform release redirect")
		}
		if response.Request.URL.Path != index || response.StatusCode != http.StatusOK {
			return nil
		}

		body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		if err != nil {
			return fmt.Errorf("read Terraform release metadata: %w", err)
		}
		response.Body.Close()
		if err := validateReleaseMetadata(body, want); err != nil {
			return err
		}
		response.Body = io.NopCloser(bytes.NewReader(body))
		return nil
	}
	proxy.ErrorHandler = func(writer http.ResponseWriter, _ *http.Request, err error) {
		http.Error(writer, "invalid pinned Terraform release: "+err.Error(), http.StatusBadRequest)
	}

	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		path := request.URL.Path
		if !strings.HasPrefix(path, prefix) {
			http.Error(writer, "Terraform artifact does not match the pinned release", http.StatusBadRequest)
			return
		}

		artifact := strings.TrimPrefix(path, prefix)
		if artifact != "index.json" && artifact != checksums && artifact != archive && !validSignature(artifact, checksums) {
			http.Error(writer, "unexpected Terraform release artifact", http.StatusBadRequest)
			return
		}
		proxy.ServeHTTP(writer, request)
	})
}

type releaseMetadata struct {
	Name       string         `json:"name"`
	Version    string         `json:"version"`
	Checksums  string         `json:"shasums"`
	Signature  string         `json:"shasums_signature"`
	Signatures []string       `json:"shasums_signatures"`
	Builds     []releaseBuild `json:"builds"`
}

type releaseBuild struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Filename string `json:"filename"`
	URL      string `json:"url"`
}

func validateReleaseMetadata(body []byte, want string) error {
	var metadata releaseMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return fmt.Errorf("decode Terraform release metadata: %w", err)
	}
	if metadata.Name != "terraform" || metadata.Version != want {
		return fmt.Errorf("Terraform release metadata identifies %s %s, expected terraform %s", metadata.Name, metadata.Version, want)
	}

	checksums := "terraform_" + want + "_SHA256SUMS"
	if metadata.Checksums != checksums {
		return fmt.Errorf("Terraform checksum manifest %q does not match pinned release", metadata.Checksums)
	}
	if metadata.Signature != "" && !validSignature(metadata.Signature, checksums) {
		return fmt.Errorf("Terraform checksum signature %q does not match pinned release", metadata.Signature)
	}
	if metadata.Signature == "" && len(metadata.Signatures) == 0 {
		return fmt.Errorf("Terraform release metadata has no checksum signature")
	}
	for _, signature := range metadata.Signatures {
		if !validSignature(signature, checksums) {
			return fmt.Errorf("Terraform checksum signature %q does not match pinned release", signature)
		}
	}

	archive := "terraform_" + want + "_" + runtime.GOOS + "_" + runtime.GOARCH + ".zip"
	archiveURL := "https://releases.hashicorp.com/terraform/" + want + "/" + archive
	matchedBuilds := 0
	for _, build := range metadata.Builds {
		if build.OS != runtime.GOOS || build.Arch != runtime.GOARCH {
			continue
		}
		matchedBuilds++
		if build.Name != "terraform" || build.Version != want || build.Filename != archive || build.URL != archiveURL {
			return fmt.Errorf("Terraform %s/%s build does not match pinned release %s", runtime.GOOS, runtime.GOARCH, want)
		}
	}
	if matchedBuilds != 1 {
		return fmt.Errorf("expected one Terraform %s/%s build, found %d", runtime.GOOS, runtime.GOARCH, matchedBuilds)
	}
	return nil
}

func validSignature(filename, checksums string) bool {
	if filename == checksums+".sig" {
		return true
	}
	if !strings.HasPrefix(filename, checksums+".") || !strings.HasSuffix(filename, ".sig") {
		return false
	}
	keyID := strings.TrimSuffix(strings.TrimPrefix(filename, checksums+"."), ".sig")
	if keyID == "" {
		return false
	}
	for _, character := range keyID {
		if !strings.ContainsRune("0123456789abcdefABCDEF", character) {
			return false
		}
	}
	return true
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
