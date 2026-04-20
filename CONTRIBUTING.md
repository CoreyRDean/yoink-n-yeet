# Contributing

Thanks for the interest. yoink-n-yeet welcomes issues and pull requests from
anyone.

## Filing an issue

- Check [existing issues](https://github.com/CoreyRDean/yoink-n-yeet/issues) first.
- Use the bug report or feature request template.
- For security issues, see [SECURITY.md](./SECURITY.md) instead — don't open a public issue.

## Opening a pull request

1. Fork the repo (or clone it if you have push access).
2. Create a topic branch from `main`.
3. Make focused, logically-atomic commits. Keep them small and self-explanatory.
4. Add or update tests for behavior changes. `go test ./...` must pass.
5. Run `golangci-lint run` locally; CI will run it too.
6. Push, open a PR against `main`, and request review from `@CoreyRDean`.
7. A PR needs a passing CI run and one approving review before it can merge.

We use **squash** or **rebase** merges only — no merge commits on `main`.

## Commit & PR style

- Imperative, present-tense subject under 72 characters.
- Body (optional) explains *why*, not *what*; the diff shows what.
- PR descriptions should be short and focused on intent. See the PR template.
- If an LLM agent assisted, add `Co-Authored-By: Oz <oz-agent@warp.dev>` to the commit and PR body.

## Local development

```sh
# Build and test
go build ./cmd/yoink-n-yeet
go test ./...

# Install locally (channel = local)
./install.sh --local

# Uninstall
./uninstall.sh
# or
yk --uninstall
```

When installed with `--local`, `yk --version` reports the repo path and
current commit (with a `+dirty` marker if your working tree has uncommitted
changes). Running `yk --update` on a local install re-runs `install.sh
--local` against whatever is currently checked out.

## Scope guardrails

- Read [INTENT.md](./INTENT.md). PRs that conflict with the stated intent or
  non-goals will be closed with an explanation.
- Keep the binary zero-dep where possible. New Go module dependencies need
  justification in the PR body.
- Don't mix unrelated changes. One concern per PR.

## Code of conduct

Be decent. See [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md).
