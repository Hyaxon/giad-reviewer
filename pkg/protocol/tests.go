package protocol

// TestRequest selects trusted host configuration. Agents cannot supply commands,
// arguments, paths, environment variables, or timeouts.
type TestRequest struct {
	Profile string `json:"profile"`
}

// TestResult is host-observed evidence, also retained in the local draft output.
// ExitCode is null when the profile deadline interrupted execution.
type TestResult struct {
	Profile    string `json:"profile"`
	ExitCode   *int   `json:"exitCode"`
	Output     string `json:"output"`
	DurationMS int64  `json:"durationMs"`
	TimedOut   bool   `json:"timedOut"`
	Truncated  bool   `json:"truncated"`
	OOMKilled  bool   `json:"oomKilled"`
}
