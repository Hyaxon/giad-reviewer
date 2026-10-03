// Package protocol defines the language-independent giad/v1 wire contract.
// Agents own their prompts and reasoning. The runtime owns these transport types.
package protocol

import "encoding/json"

const Version = "giad/v1"
const MaxMessageBytes = 1024 * 1024

// Frames are newline-delimited JSON, with one outstanding request at a time.
// The host sends review.start; agents request capabilities and finally review.finish.
type Frame struct {
	APIVersion string          `json:"apiVersion"`
	ID         string          `json:"id,omitempty"`
	Method     string          `json:"method,omitempty"`
	Params     json.RawMessage `json:"params,omitempty"`
	Result     json.RawMessage `json:"result,omitempty"`
	Error      string          `json:"error,omitempty"`
}

type Manifest struct {
	APIVersion    string       `json:"apiVersion"`
	Name          string       `json:"name"`
	Version       string       `json:"version"`
	Entrypoint    Entrypoint   `json:"entrypoint"`
	Capabilities  Capabilities `json:"capabilities"`
	ModelProfiles []string     `json:"modelProfiles"`
}
type Entrypoint struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}
type Capabilities struct {
	Required []string `json:"required"`
	Optional []string `json:"optional"`
}

type Job struct {
	Repository          string        `json:"repository"`
	Number              int           `json:"number"`
	URL                 string        `json:"url"`
	Title               string        `json:"title"`
	Body                string        `json:"body"`
	BaseSHA             string        `json:"baseSHA"`
	HeadSHA             string        `json:"headSHA"`
	ChangedFiles        []ChangedFile `json:"changedFiles"`
	LinkedIssues        []Issue       `json:"linkedIssues"`
	IssuesError         string        `json:"issuesError,omitempty"`
	TrustedInstructions []Instruction `json:"trustedInstructions"`
	AllowedCapabilities []string      `json:"allowedCapabilities"`
	ModelProfiles       []string      `json:"modelProfiles"`
	TestProfiles        []string      `json:"testProfiles"`
}
type ChangedFile struct {
	Path         string `json:"path"`
	PreviousPath string `json:"previousPath,omitempty"`
	Status       string `json:"status"`
}
type Issue struct {
	Repository string `json:"repository"`
	Number     int    `json:"number"`
	URL        string `json:"url"`
	Title      string `json:"title"`
	Body       string `json:"body"`
}
type Instruction struct {
	Path    string `json:"path"`
	Scope   string `json:"scope"`
	BaseSHA string `json:"baseSHA"`
	Content string `json:"content"`
}
type Report struct {
	Summary     string    `json:"summary"`
	Limitations string    `json:"limitations"`
	Findings    []Finding `json:"findings"`
}
type Finding struct {
	Source          string  `json:"source"`
	Category        string  `json:"category"`
	File            string  `json:"file"`
	Line            int     `json:"line"`
	Severity        string  `json:"severity"`
	Confidence      float64 `json:"confidence"`
	Title           string  `json:"title"`
	Explanation     string  `json:"explanation"`
	Evidence        string  `json:"evidence"`
	FailureScenario string  `json:"failure_scenario"`
	SuggestedFix    string  `json:"suggested_fix"`
}
