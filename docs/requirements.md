# Linked issues and requirements traceability

Status: accepted V1 design, implemented after the first PR acquisition slice.

## Authoritative context

BALTHASAR checks whether a PR meets requirements in formally linked GitHub
issues. Use GitHub's relationship data, including manual Development-sidebar
links, rather than treating branch names or arbitrary issue mentions as requirements.

The PDF proposes the GraphQL field `PullRequest.closingIssuesReferences`:

```graphql
query PullRequestIssues($owner: String!, $repo: String!, $number: Int!) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      closingIssuesReferences(first: 10) {
        nodes {
          number
          title
          bodyText
          url
          repository { nameWithOwner }
        }
        pageInfo { hasNextPage endCursor }
      }
    }
  }
}
```

This is an initial query shape, not a complete pagination implementation. Follow
additional pages when present or report an explicit retrieval limit; do not silently
truncate requirements. Verify the exact query with an installation token.

Retain repository, issue number, URL, title, body, and available link provenance.
Cross-repository private issues require access to that repository too. Distinguish
"no formal link" from "linked context could not be retrieved."

## Requirement extraction

| Source | Meaning | Treatment |
| --- | --- | --- |
| `explicit` | Checkbox, numbered acceptance criterion, labeled requirement | Authoritative unless clearly superseded |
| `stated` | Unambiguous requirement in prose | Preserve source evidence and evaluate |
| `inferred` | Behavior guessed by the model | Context only by default; not a blocking finding |

Each requirement retains an ID, issue reference, text, source classification, and
confidence. Preserve the source excerpt/location so interpretation is auditable.
Scope IDs to the issue, for example `owner/repo#123:R1`.

Start with the title and body. Issue comments are disabled by default because they
can contain outdated proposals and brainstorming. A later comment-ingestion feature
must preserve chronology and explicit decisions.

## Assessment

| Status | Meaning | Output |
| --- | --- | --- |
| `satisfied` | Code and/or tests support the requirement | Coverage summary with evidence |
| `unsatisfied` | Evidence shows a requirement is not met | Finding when sufficiently verified |
| `unclear` | Evidence is incomplete or behavior is externally controlled | Explain uncertainty without claiming a defect |

Absence of evidence is not automatically evidence of absence. Infrastructure,
another service, or an inaccessible repository may determine behavior. Use `unclear`
when this checkout cannot establish the answer.

Evaluate linked issues separately rather than blending their requirements into
one prompt. A PR may satisfy one issue and leave another unresolved.

## Missing links and publication

If no formal link exists, say so and skip authoritative requirement compliance.
Normal code review continues. Optional heuristic references may be shown for human
confirmation, but are not automatically promoted to the specification.

BALTHASAR publishes a concise coverage summary. High-confidence unsatisfied
requirements may become inline findings only when they map to a defensible changed
line or range. Otherwise report them in the summary. Never invent a line solely to
make a requirement appear as a merge-blocking thread.

Planned flags such as `--require-linked-issue` must distinguish missing links from
API errors. See [CLI design](cli.md) and [sources](sources.md).
