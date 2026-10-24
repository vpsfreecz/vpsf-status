package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetVendorHash(t *testing.T) {
	const oldHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	const newHash = "sha256-//////////++++++++++aaaaaaaaaaaaaaaaaaaaaaa="
	const overrideHash = "sha256-N6vS4SGVeWkBc23/z8zT1Hbz+MC7BvOsjqJkw+bXcgI="
	conditionalPrefix := "  vendorHash =\n    if vpsadminGoClientSource != null then\n      \"" + overrideHash + "\"\n    else\n      "
	tests := []struct {
		name, input, hash, want string
		fail                    bool
	}{
		{"flat hash to fake", "vendorHash = \"" + oldHash + "\";\n", "--fake", "vendorHash = lib.fakeHash;\n", false},
		{"flat fake to base64 hash", "vendorHash = lib.fakeHash;\n", newHash, "vendorHash = \"" + newHash + "\";\n", false},
		{"conditional hash to fake", conditionalPrefix + "\"" + oldHash + "\";\n", "--fake", conditionalPrefix + "lib.fakeHash;\n", false},
		{"conditional fake to base64 hash", conditionalPrefix + "lib.fakeHash;\n", newHash, conditionalPrefix + "\"" + newHash + "\";\n", false},
		{"conditional hash to base64 hash", conditionalPrefix + "\"" + oldHash + "\";\n", newHash, conditionalPrefix + "\"" + newHash + "\";\n", false},
		{"unknown expression", "vendorHash = anotherHash;\n", newHash, "", true},
		{"multiple assignments", "vendorHash = lib.fakeHash;\nvendorHash = lib.fakeHash;\n", newHash, "", true},
		{"malformed hash", "vendorHash = lib.fakeHash;\n", "sha256-short/+=", "", true},
		{"hash contains code", "vendorHash = lib.fakeHash;\n", newHash + "\"; die 'injected';", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "package.nix")
			if err := os.WriteFile(path, []byte(tt.input), 0o600); err != nil {
				t.Fatal(err)
			}
			output, err := exec.Command("perl", "set-vendor-hash.pl", tt.hash, path).CombinedOutput()
			if (err != nil) != tt.fail {
				t.Fatalf("error = %v, want failure %v; output: %s", err, tt.fail, output)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want := tt.want
			if tt.fail {
				want = tt.input
			}
			if string(got) != want {
				t.Fatalf("file = %q, want %q", got, want)
			}
		})
	}
}

func TestSetVendorHashCurrentPackage(t *testing.T) {
	source, err := os.ReadFile("../nix/package.nix")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "package.nix")
	if err := os.WriteFile(path, source, 0o600); err != nil {
		t.Fatal(err)
	}
	const hash = "sha256-//////////++++++++++aaaaaaaaaaaaaaaaaaaaaaa="
	for _, value := range []string{"--fake", hash} {
		if output, err := exec.Command("perl", "set-vendor-hash.pl", value, path).CombinedOutput(); err != nil {
			t.Fatalf("%s: %v: %s", value, err, output)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	before, _, ok := strings.Cut(string(source), "    else\n")
	if !ok || !strings.HasPrefix(string(got), before+"    else\n") {
		t.Fatalf("local-source override changed: %s", got)
	}
	if strings.Count(string(got), "\""+hash+"\"") != 1 || strings.Contains(string(got), "lib.fakeHash") {
		t.Fatalf("default vendor hash was not installed: %s", got)
	}
}
