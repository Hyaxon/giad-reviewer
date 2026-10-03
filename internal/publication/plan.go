// Package publication prepares human-confirmed GitHub reviews from saved drafts.
package publication

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/hyaxon/giad/internal/githubapi"
	"github.com/hyaxon/giad/internal/githubauth"
	"github.com/hyaxon/giad/internal/review"
	"github.com/hyaxon/giad/pkg/protocol"
)

type Plan struct {
	Repository                  githubauth.Repository
	Number                      int
	BaseSHA, HeadSHA, Body, Key string
	Event                       string
	Comments                    []githubapi.InlineComment
	legacy                      *Plan
}

type Options struct {
	Event  string
	Inline bool
}

func (o Options) Normalize() (Options, error) {
	o.Event = strings.ReplaceAll(strings.ToUpper(o.Event), "-", "_")
	if o.Event == "" {
		o.Event = "COMMENT"
	}
	if _, err := githubapi.ReviewState(o.Event); err != nil {
		return o, err
	}
	return o, nil
}

func LoadDraft(path string) (review.Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return review.Result{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 1024*1024+1))
	if err != nil {
		return review.Result{}, err
	}
	if len(data) > 1024*1024 {
		return review.Result{}, errors.New("draft exceeds 1 MiB")
	}
	var draft review.Result
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&draft); err != nil {
		return draft, fmt.Errorf("decode saved draft: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return draft, errors.New("expected one saved draft")
	}
	if draft.APIVersion != protocol.Version {
		return draft, errors.New("draft requires apiVersion giad/v1; generate a fresh review --json")
	}
	return draft, nil
}

var shaPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)
var repoPart = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

func Target(draft review.Result) (githubauth.Repository, error) {
	parts := strings.Split(draft.Job.Repository, "/")
	if len(parts) != 2 || !repoPart.MatchString(parts[0]) || !repoPart.MatchString(parts[1]) || parts[0] == "." || parts[0] == ".." || parts[1] == "." || parts[1] == ".." || draft.Job.Number <= 0 {
		return githubauth.Repository{}, errors.New("draft requires a valid repository and positive PR number")
	}
	return githubauth.Repository{Host: "github.com", Owner: strings.ToLower(parts[0]), Name: strings.ToLower(parts[1])}, nil
}

// Prepare keeps the original body-only COMMENT behavior for existing callers.
func Prepare(draft review.Result, current githubapi.PRContext) (Plan, error) {
	return PrepareWithOptions(draft, current, Options{})
}

// PrepareWithOptions validates every inline anchor against fresh head-side diff
// hunks. The human chooses the event and placement, independently of the agent.
func PrepareWithOptions(draft review.Result, current githubapi.PRContext, options Options) (Plan, error) {
	plan, err := prepareWithRenderer(draft, current, options, text, false)
	if err != nil {
		return Plan{}, err
	}
	// Keep the exact old formatting for read-only retry reconciliation.
	legacy, err := prepareWithRenderer(draft, current, options, legacyText, true)
	if err != nil {
		return Plan{}, err
	}
	if legacy.Key != plan.Key {
		plan.legacy = &legacy
	}
	previous, err := prepareWithRenderer(draft, current, options, text, true)
	if err != nil {
		return Plan{}, err
	}
	if previous.Key != plan.Key && previous.Key != legacy.Key {
		legacy.legacy = &previous
	}
	return plan, nil
}

func prepareWithRenderer(draft review.Result, current githubapi.PRContext, options Options, text func(string) string, showRevisions bool) (Plan, error) {
	options, err := options.Normalize()
	if err != nil {
		return Plan{}, err
	}
	if draft.APIVersion != protocol.Version {
		return Plan{}, errors.New("unsupported saved draft version")
	}
	repo, err := Target(draft)
	if err != nil {
		return Plan{}, err
	}
	job := draft.Job
	if !shaPattern.MatchString(job.BaseSHA) || !shaPattern.MatchString(job.HeadSHA) || current.PullRequest.Number != job.Number || current.PullRequest.Base.SHA != job.BaseSHA || current.PullRequest.Head.SHA != job.HeadSHA {
		return Plan{}, errors.New("draft revisions do not match the current PR; run a new review")
	}
	if current.PullRequest.State != "open" {
		return Plan{}, errors.New("publication requires an open PR")
	}
	if draft.Agent == "" || len(draft.Agent) > 128 || strings.TrimSpace(draft.Report.Summary) == "" || len(draft.Report.Summary) > 8192 || len(draft.Report.Limitations) > 16384 || draft.Report.Findings == nil || len(draft.Report.Findings) > 20 {
		return Plan{}, errors.New("invalid or oversized draft report")
	}
	anchors, err := diffAnchors(current.Diff)
	if err != nil {
		return Plan{}, err
	}
	changed := map[string]bool{}
	for _, file := range current.Files {
		changed[file.Filename] = file.Status != "removed"
	}
	var body strings.Builder
	var comments []githubapi.InlineComment
	fmt.Fprintf(&body, "## GIAD review — %s\n\n", text(draft.Agent))
	if showRevisions {
		fmt.Fprintf(&body, "Base: `%s`\nHead: `%s`\n\n", job.BaseSHA, job.HeadSHA)
	}
	fmt.Fprintf(&body, "%s\n", text(draft.Report.Summary))
	for _, f := range draft.Report.Findings {
		anchored := false
		for _, span := range anchors[f.File] {
			anchored = anchored || (f.Line >= span[0] && f.Line < span[1])
		}
		if !changed[f.File] || !anchored {
			return Plan{}, fmt.Errorf("finding is outside the current head diff: %s:%d", f.File, f.Line)
		}
		if (f.Severity != "high" && f.Severity != "medium" && f.Severity != "low") || f.Confidence < 0 || f.Confidence > 1 {
			return Plan{}, errors.New("invalid finding severity/confidence")
		}
		for _, value := range []string{f.Source, f.Category, f.Title, f.Explanation, f.Evidence, f.FailureScenario, f.SuggestedFix} {
			if strings.TrimSpace(value) == "" || len(value) > 8192 {
				return Plan{}, errors.New("invalid finding fields")
			}
		}
		findingBody := fmt.Sprintf("\n### [%s] %s\n\n%s:%d (confidence %.2f)\n\n%s\n\nEvidence: %s\n\nScenario: %s\n\nSuggested fix: %s\n", f.Severity, text(f.Title), text(f.File), f.Line, f.Confidence, text(f.Explanation), text(f.Evidence), text(f.FailureScenario), text(f.SuggestedFix))
		if options.Inline {
			if len(findingBody) > 60000 {
				return Plan{}, errors.New("inline comment exceeds 60000 bytes")
			}
			comments = append(comments, githubapi.InlineComment{Path: f.File, Line: f.Line, Side: "RIGHT", Body: findingBody})
		} else {
			body.WriteString(findingBody)
		}
	}
	if len(draft.Report.Findings) == 0 {
		body.WriteString("\nNo findings reported; this is not proof of correctness.\n")
	}
	if len(comments) > 0 {
		fmt.Fprintf(&body, "\n%d findings attached as inline comments.\n", len(comments))
	}
	fmt.Fprintf(&body, "\n### Coverage limitations\n\n%s\n", text(draft.Report.Limitations))
	if len(draft.TestRuns) == 0 {
		body.WriteString("\nGIAD recorded no test runs.\n")
	}
	for _, run := range draft.TestRuns {
		status := "interrupted"
		if run.TimedOut {
			status = "timed out"
		} else if run.ExitCode != nil {
			status = fmt.Sprintf("exit %d", *run.ExitCode)
		}
		fmt.Fprintf(&body, "\nTest profile %s: %s, %d ms (head only).", text(run.Profile), status, run.DurationMS)
		if run.Truncated {
			body.WriteString(" Output truncated.")
		}
		if run.OOMKilled {
			body.WriteString(" Memory limit exceeded.")
		}
		body.WriteByte('\n')
	}
	if len(draft.TestRuns) > 2 || len(body.String()) > 60000 {
		return Plan{}, errors.New("publication body exceeds limits")
	}
	identity := fmt.Sprintf("%s/%s#%d\n%s\n%s\n%s", repo.Owner, repo.Name, job.Number, job.BaseSHA, job.HeadSHA, body.String())
	// Preserve existing COMMENT hashes; new modes bind the action and every inline
	// path/line/side/body so a previous confirmation cannot authorize other writes.
	if options.Event != "COMMENT" || options.Inline {
		data, err := json.Marshal(struct {
			Event    string
			Inline   bool
			Comments []githubapi.InlineComment
		}{options.Event, options.Inline, comments})
		if err != nil {
			return Plan{}, err
		}
		if len(data)+len(body.String()) > 1024*1024 {
			return Plan{}, errors.New("publication exceeds 1 MiB")
		}
		identity += "\n" + string(data)
	}
	sum := sha256.Sum256([]byte(identity))
	key := hex.EncodeToString(sum[:])
	fmt.Fprintf(&body, "\n<!-- giad-publication:%s -->\n", key)
	return Plan{Repository: repo, Number: job.Number, BaseSHA: job.BaseSHA, HeadSHA: job.HeadSHA, Body: body.String(), Key: key, Event: options.Event, Comments: comments}, nil
}

// legacyText is retained only to recognize previous publication attempts.
func legacyText(value string) string {
	value = html.EscapeString(value)
	replacer := strings.NewReplacer("\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "#", "\\#", "!", "\\!", "@", "&#64;")
	return replacer.Replace(value)
}

var hunkPattern = regexp.MustCompile(`^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@`)

func diffAnchors(diff string) (map[string][][2]int, error) {
	anchors := map[string][][2]int{}
	scanner := bufio.NewScanner(strings.NewReader(diff))
	scanner.Buffer(make([]byte, 4096), 16*1024*1024)
	file := ""
	header := false
	oldLeft, newLeft, next := 0, 0, 0
	fail := func() (map[string][][2]int, error) { return nil, errors.New("malformed or incomplete unified diff") }
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "\\ No newline") {
			continue
		}
		if oldLeft > 0 || newLeft > 0 {
			if line == "" {
				return fail()
			}
			switch line[0] {
			case ' ':
				oldLeft--
				newLeft--
				next++
			case '+':
				newLeft--
				next++
			case '-':
				oldLeft--
			default:
				return fail()
			}
			if oldLeft < 0 || newLeft < 0 {
				return fail()
			}
			continue
		}
		if strings.HasPrefix(line, "diff --git ") {
			file = ""
			header = false
			continue
		}
		if strings.HasPrefix(line, "+++ ") {
			header = true
			name := strings.TrimPrefix(line, "+++ ")
			if strings.HasPrefix(name, "\"") {
				var err error
				name, err = strconv.Unquote(name)
				if err != nil {
					return fail()
				}
			}
			if name == "/dev/null" {
				file = ""
				continue
			}
			if !strings.HasPrefix(name, "b/") || !fs.ValidPath(strings.TrimPrefix(name, "b/")) {
				return fail()
			}
			file = strings.TrimPrefix(name, "b/")
			continue
		}
		if strings.HasPrefix(line, "@@") {
			match := hunkPattern.FindStringSubmatch(line)
			if match == nil || !header {
				return fail()
			}
			values := []int{0, 1, 0, 1}
			for i := 1; i <= 4; i++ {
				if match[i] != "" {
					n, err := strconv.Atoi(match[i])
					if err != nil || n > 1_000_000_000 {
						return fail()
					}
					values[i-1] = n
				}
			}
			oldLeft, newLeft, next = values[1], values[3], values[2]
			if file == "" && newLeft != 0 {
				return fail()
			}
			if newLeft > 0 && next < 1 {
				return fail()
			}
			if file != "" {
				anchors[file] = append(anchors[file], [2]int{next, next + newLeft})
			}
		}
	}
	if scanner.Err() != nil || oldLeft != 0 || newLeft != 0 {
		return fail()
	}
	return anchors, nil
}
