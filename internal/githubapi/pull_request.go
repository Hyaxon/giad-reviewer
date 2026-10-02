package githubapi

type PullRequest struct {
	ChangedFiles int    `json:"changed_files"`
	Number       int    `json:"number"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	URL          string `json:"html_url"`
	Base         Ref    `json:"base"`
	Head         Ref    `json:"head"`
}

type Ref struct {
	Name string `json:"ref"`
	SHA  string `json:"sha"`
}
