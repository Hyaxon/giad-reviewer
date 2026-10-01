# Password reset validation

The reset endpoint accepts a token and an account ID, validates the request,
then updates the password for the account returned by `AuthorizeReset`.
Token lookup happens before validation. After a successful password update,
the stored token is marked as used.

## Expected behavior

- Tokens are valid only before their expiration timestamp.
- A token can reset only the account it was issued for.
- A token must not be accepted again after a successful password reset.

## Validation helper

```go
package reset

import (
    "errors"
    "time"
)

var (
    ErrExpired = errors.New("reset token expired")
    ErrUsed    = errors.New("reset token already used")
)

type Token struct {
    AccountID string
    ExpiresAt time.Time
    Used      bool
}

func AuthorizeReset(token Token, accountID string, now time.Time) (string, error) {
    if now.After(token.ExpiresAt) {
        return "", ErrExpired
    }
    if token.Used {
        return "", ErrUsed
    }
    return accountID, nil
}
```

## Tests

```go
package reset

import (
    "errors"
    "testing"
    "time"
)

func TestAuthorizeReset(t *testing.T) {
    expiresAt := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)
    token := Token{AccountID: "account-123", ExpiresAt: expiresAt}

    t.Run("valid token", func(t *testing.T) {
        accountID, err := AuthorizeReset(token, "account-123", expiresAt.Add(-time.Second))
        if err != nil {
            t.Fatal(err)
        }
        if accountID != "account-123" {
            t.Fatalf("account ID = %q, want account-123", accountID)
        }
    })

    t.Run("expired token", func(t *testing.T) {
        _, err := AuthorizeReset(token, "account-123", expiresAt.Add(time.Second))
        if !errors.Is(err, ErrExpired) {
            t.Fatalf("error = %v, want ErrExpired", err)
        }
    })

    t.Run("used token", func(t *testing.T) {
        usedToken := token
        usedToken.Used = true
        _, err := AuthorizeReset(usedToken, "account-123", expiresAt.Add(time.Second))
        if err == nil {
            t.Fatal("expected used token to be rejected")
        }
    })
}
```
