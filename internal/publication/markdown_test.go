package publication

import (
	"strings"
	"testing"
)

func TestPublicationCodeFormatting(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"The `AuthorizeReset` function returns a different error.", "The `AuthorizeReset` function returns a different error."},
		{"Use ``value containing `backticks` `` here.", "Use ``value containing `backticks` `` here."},
		{"```go\nif token.AccountID != accountID {\n    return errors.New(\"mismatch\")\n}\n```", "\n\n```go\nif token.AccountID != accountID {\n    return errors.New(\"mismatch\")\n}\n```\n\n"},
		{"~~~python\nprint('ok')\n~~~", "\n\n~~~python\nprint('ok')\n~~~\n\n"},
		{"Before\n```text\n<script>@user [link](url)</script>\n```\nAfter", "Before\n\n\n```text\n<script>@user [link](url)</script>\n```\n\n\nAfter"},
		{"It's \"quoted\".", "It&#39;s &#34;quoted&#34;."},
		{"`<script>@user [link](url)</script>`", "`<script>@user [link](url)</script>`"},
	} {
		if got := text(tc.input); got != tc.want {
			t.Errorf("input %q: got %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestPublicationProseRemainsLiteral(t *testing.T) {
	for _, input := range []string{
		"<script>@user [link](https://example.test)</script>",
		"<https://example.test> ![image](https://example.test)",
		"A `missing close with <script>@user",
		"```go\n<script>@user\nmissing closing fence",
		"~~~\n<script>@user\nmissing closing fence",
	} {
		got := text(input)
		for _, forbidden := range []string{"<script>", "<https://", "@user", "[link](", "![image](", "&\\#39;"} {
			if strings.Contains(got, forbidden) {
				t.Errorf("untrusted prose escaped suppression: %q", got)
			}
		}
	}
}

func TestPlanPreservesCodeInBodyAndInlineComments(t *testing.T) {
	for _, inline := range []bool{false, true} {
		draft, current := fixture()
		draft.Report.Findings[0].FailureScenario = "The `AuthorizeReset` function returns a different error."
		draft.Report.Findings[0].SuggestedFix = "```go\nreturn errors.New(\"mismatch\")\n```"
		plan, err := PrepareWithOptions(draft, current, Options{Inline: inline})
		if err != nil {
			t.Fatal(err)
		}
		body := plan.Body
		if inline {
			body = plan.Comments[0].Body
		}
		if !strings.Contains(body, "`AuthorizeReset`") || strings.Contains(body, "\\`") || !strings.Contains(body, "Suggested fix: \n\n```go\n") || !strings.Contains(body, "return errors.New(\"mismatch\")") {
			t.Fatalf("code formatting damaged in publication: %s", body)
		}
		if plan.legacy == nil || plan.Key == plan.legacy.Key {
			t.Fatal("old publication identity was not retained")
		}
		oldBody := plan.legacy.Body
		if inline {
			oldBody = plan.legacy.Comments[0].Body
		}
		if !strings.Contains(oldBody, "\\`AuthorizeReset\\`") {
			t.Fatal("old rendering was not preserved exactly")
		}
	}
}
