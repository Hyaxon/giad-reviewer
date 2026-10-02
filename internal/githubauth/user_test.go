package githubauth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestUserAuthToken(t *testing.T) {
	auth := UserAuth{run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name != "gh" || !reflect.DeepEqual(args, []string{"auth", "token", "--hostname", "github.com"}) {
			t.Fatalf("unexpected credential command: %s %v", name, args)
		}
		return []byte("fake-token\n"), nil
	}}
	token, err := auth.Token(context.Background(), Repository{})
	if err != nil || token != "fake-token" {
		t.Fatal("failed to retrieve trimmed token")
	}
	if _, err := auth.Token(context.Background(), Repository{Host: "other.example"}); err == nil {
		t.Fatal("expected unsupported host rejection")
	}
}

func TestUserAuthFailureDoesNotExposeOutput(t *testing.T) {
	auth := UserAuth{run: func(context.Context, string, ...string) ([]byte, error) {
		return []byte("secret-value"), errors.New("secret-value")
	}}
	_, err := auth.Token(context.Background(), Repository{})
	if err == nil || strings.Contains(err.Error(), "secret-value") || !strings.Contains(err.Error(), "gh auth login") {
		t.Fatal("expected safe login guidance")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestVerifyUser(t *testing.T) {
	auth := UserAuth{run: func(context.Context, string, ...string) ([]byte, error) {
		return []byte("fake-token"), nil
	}}
	for _, status := range []int{http.StatusOK, http.StatusUnauthorized} {
		client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://api.github.com/user" || req.Method != http.MethodGet || req.Header.Get("Authorization") != "Bearer fake-token" {
				t.Fatal("incorrect identity request")
			}
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"login":"alice"}`)), Header: make(http.Header)}, nil
		})}
		login, err := verifyUser(context.Background(), auth, client)
		if status == http.StatusOK {
			if err != nil || login != "alice" {
				t.Fatalf("identity = %q, error = %v", login, err)
			}
		} else if err == nil || login != "" {
			t.Fatal("expected rejected credentials to fail verification")
		}
	}
}
