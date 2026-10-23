package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func digest(key string) string { s := sha256.Sum256([]byte(key)); return hex.EncodeToString(s[:]) }

func write(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "keys")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAuthenticate(t *testing.T) {
	ks, err := Load(write(t, "# comment\n\nacme "+digest("secret-a")+"\nglobex "+digest("secret-b")+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ks.Authenticate("secret-b"); err != nil || got != "globex" {
		t.Fatalf("got %q, %v", got, err)
	}
	for _, bad := range []string{"", "nope", "secret-a "} {
		if _, err := ks.Authenticate(bad); err != ErrUnauthenticated {
			t.Errorf("key %q: want ErrUnauthenticated, got %v", bad, err)
		}
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"empty":         "",
		"bad fields":    "acme\n",
		"bad tenant":    "Acme " + digest("x") + "\n",
		"bad digest":    "acme zz\n",
		"short digest":  "acme abcd\n",
		"duplicate key": "a " + digest("x") + "\nb " + digest("x") + "\n",
	}
	for name, content := range cases {
		if _, err := Load(write(t, content)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestReloadKeepsOldKeysOnError(t *testing.T) {
	p := write(t, "acme "+digest("k1")+"\n")
	ks, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("garbage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ks.Reload(); err == nil {
		t.Fatal("expected reload error")
	}
	if _, err := ks.Authenticate("k1"); err != nil {
		t.Fatalf("old key should still work: %v", err)
	}
	if err := os.WriteFile(p, []byte(strings.Join([]string{"acme " + digest("k2")}, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ks.Reload(); err != nil {
		t.Fatal(err)
	}
	if _, err := ks.Authenticate("k1"); err == nil {
		t.Fatal("revoked key must stop working")
	}
}
