// Package githubauth supplies credentials independently of review logic.
package githubauth

import "context"

// Repository identifies the target for providers with repository-scoped tokens.
// Only github.com is supported initially; an empty Host means github.com.
type Repository struct {
	Host  string
	Owner string
	Name  string
}

// Provider returns a token for trusted GitHub API code, never for model tools.
type Provider interface {
	Token(context.Context, Repository) (string, error)
}

// AuthorProvider identifies publishers whose tokens do not support GET /user.
type AuthorProvider interface {
	ReviewAuthor(context.Context, Repository) (int64, error)
}
