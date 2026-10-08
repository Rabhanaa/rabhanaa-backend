package secretbox

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

func newKey(t *testing.T) string {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(k)
}

func TestRoundTrip(t *testing.T) {
	box, err := New(newKey(t))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := box.Seal("sk-secret-value")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sealed, "sk-secret-value") {
		t.Fatal("sealed value contains the plaintext")
	}
	again, _ := box.Seal("sk-secret-value")
	if again == sealed {
		t.Fatal("sealing twice gave identical output; nonce is not random")
	}
	plain, err := box.Open(sealed)
	if err != nil || plain != "sk-secret-value" {
		t.Fatalf("Open = %q, %v", plain, err)
	}
}

func TestWrongKeyAndTampering(t *testing.T) {
	a, _ := New(newKey(t))
	b, _ := New(newKey(t))
	sealed, _ := a.Seal("value")
	if _, err := b.Open(sealed); err == nil {
		t.Fatal("opened with the wrong key")
	}
	tampered := sealed[:len(sealed)-2] + "AA"
	if _, err := a.Open(tampered); err == nil {
		t.Fatal("opened a tampered value")
	}
	if _, err := a.Open("plain-text"); err != ErrMalformed {
		t.Fatalf("got %v, want ErrMalformed", err)
	}
}

func TestBadKeys(t *testing.T) {
	for _, k := range []string{"", "not base64!", base64.StdEncoding.EncodeToString([]byte("short"))} {
		if _, err := New(k); err == nil {
			t.Fatalf("New(%q) accepted a bad key", k)
		}
	}
}
