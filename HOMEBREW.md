# Homebrew release runbook

This repo ships a Homebrew formula in the sibling tap
[`CoreyRDean/homebrew-tap`](https://github.com/CoreyRDean/homebrew-tap) at
`Formula/yoink-n-yeet.rb`. Users install via:

```sh
brew install CoreyRDean/tap/yoink-n-yeet
```

Right now the tap is **updated by hand** after every release. The goreleaser
config has a `brews:` block stubbed but commented out (`.goreleaser.yaml`,
around lines 103–131), and the `HOMEBREW_TAP_TOKEN` secret referenced by
`.github/workflows/release.yml` is not set on this repo. Until both are in
place, follow the manual runbook below.

---

## Manual bump runbook

Run after `git push origin vX.Y.Z` completes **and** the `release` workflow
finishes successfully (the tap edit depends on the release's tarballs already
existing on GitHub).

### 1. Confirm the release is live

```sh
gh release view vX.Y.Z --repo CoreyRDean/yoink-n-yeet \
  --json tagName,isDraft,isPrerelease,assets \
  --jq '{tag: .tagName, draft: .isDraft, pre: .isPrerelease, assets: [.assets[].name]}'
```

You should see the four tarballs the formula cares about:

- `yoink-n-yeet_X.Y.Z_darwin_amd64.tar.gz`
- `yoink-n-yeet_X.Y.Z_darwin_arm64.tar.gz`
- `yoink-n-yeet_X.Y.Z_linux_amd64.tar.gz`
- `yoink-n-yeet_X.Y.Z_linux_arm64.tar.gz`

### 2. Pull the signed checksums file

goreleaser publishes a single `*_checksums.txt` per release, signed with
cosign (keyless OIDC). Read the four tarball SHA256s directly from it — do
**not** re-hash the tarballs locally; trusting the signed list avoids TOCTOU
drift.

```sh
VERSION=X.Y.Z
curl -fsSL -o /tmp/yny-checksums.txt \
  "https://github.com/CoreyRDean/yoink-n-yeet/releases/download/v${VERSION}/yoink-n-yeet_${VERSION}_checksums.txt"

# Pull out the four SHAs (ignore .sbom.json and .zip rows)
grep -E "_(darwin|linux)_(amd64|arm64)\.tar\.gz$" /tmp/yny-checksums.txt
```

Optional but recommended for a first pass on a new machine — verify the
checksums file's cosign signature:

```sh
cosign verify-blob \
  --certificate      "https://github.com/CoreyRDean/yoink-n-yeet/releases/download/v${VERSION}/yoink-n-yeet_${VERSION}_checksums.txt.pem" \
  --signature        "https://github.com/CoreyRDean/yoink-n-yeet/releases/download/v${VERSION}/yoink-n-yeet_${VERSION}_checksums.txt.sig" \
  --certificate-identity-regexp "^https://github\.com/CoreyRDean/yoink-n-yeet/\.github/workflows/release\.yml@refs/tags/v${VERSION}$" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  /tmp/yny-checksums.txt
```

### 3. Edit `Formula/yoink-n-yeet.rb`

Change exactly four URLs and four SHA256s, plus the `version` line. Preserve
the rest of the file verbatim — the `argv[0]` symlinks (`yoink`, `yeet`,
`yk`, `yt`), the comment above them, and the `assert_match` test are load-
bearing for UX.

You have two ways to push the edit:

#### Option A — `gh api` (one-shot, no clone)

The Contents API requires the existing file's blob SHA for updates. Fetch
it, then PUT the new contents:

```sh
TAP=CoreyRDean/homebrew-tap
EXISTING_SHA=$(gh api "repos/${TAP}/contents/Formula/yoink-n-yeet.rb" --jq .sha)
NEW_B64=$(base64 -i ./yoink-n-yeet.rb | tr -d '\n')

gh api --method PUT "repos/${TAP}/contents/Formula/yoink-n-yeet.rb" \
  -f message="chore: bump yoink-n-yeet to v${VERSION}" \
  -f content="${NEW_B64}" \
  -f sha="${EXISTING_SHA}" \
  -f "committer[name]=Your Name" \
  -f "committer[email]=you@users.noreply.github.com"
```

#### Option B — clone and push

Clearer diffs if you want to review in your editor before shipping:

```sh
git clone https://github.com/CoreyRDean/homebrew-tap.git /tmp/homebrew-tap
$EDITOR /tmp/homebrew-tap/Formula/yoink-n-yeet.rb
git -C /tmp/homebrew-tap add Formula/yoink-n-yeet.rb
git -C /tmp/homebrew-tap commit -m "chore: bump yoink-n-yeet to v${VERSION}"
git -C /tmp/homebrew-tap push origin main
```

### 4. Smoke-test the formula

On at least one platform (the one you're on is fine — the other arch will
still be audited by `brew style`/`brew audit` if you run them):

```sh
brew untap CoreyRDean/tap 2>/dev/null; brew tap CoreyRDean/tap
brew install --build-from-source CoreyRDean/tap/yoink-n-yeet   # or skip flag
yoink-n-yeet --version   # expect vX.Y.Z
yk --version; yt --version   # symlinks dispatch correctly
brew uninstall yoink-n-yeet
```

If `brew audit --strict CoreyRDean/tap/yoink-n-yeet` flags anything new,
fix it in a follow-up commit on the tap — don't let audit regressions
accumulate.

---

## Automating it later

Three moving pieces. Do them in this order so the pipeline stays green the
whole time.

### Step 1 — provision a PAT on `CoreyRDean/homebrew-tap`

Goreleaser needs write access to the tap repo. Use a **fine-grained** PAT
scoped to that one repo, not a classic token.

1. https://github.com/settings/personal-access-tokens/new
2. Repository access → *Only select repositories* → `CoreyRDean/homebrew-tap`
3. Repository permissions:
   - **Contents: Read and write**
   - **Metadata: Read-only** (auto-added, required)
4. Expiration: pick something you'll actually rotate (90d is a sane default).
5. Generate and copy the token once — GitHub only shows it at creation.

### Step 2 — register the token as a secret on this repo

```sh
gh secret set HOMEBREW_TAP_TOKEN --repo CoreyRDean/yoink-n-yeet
# paste the PAT when prompted
gh secret list --repo CoreyRDean/yoink-n-yeet   # verify HOMEBREW_TAP_TOKEN is present
```

The release workflow (`.github/workflows/release.yml`) already forwards this
secret into goreleaser's environment — no workflow edits required.

### Step 3 — uncomment the `brews:` block in `.goreleaser.yaml`

The stub at the bottom of `.goreleaser.yaml` already has the right shape:
repo, commit author, homepage, license, symlinks, and test. Delete the
leading `#` on those lines (roughly 111–131) and commit the change on its
own:

```sh
git checkout -b chore/enable-brew-tap-publish
$EDITOR .goreleaser.yaml   # uncomment the brews: block
git commit -am "ci: let goreleaser publish the homebrew formula"
gh pr create --fill --draft
```

Open it as a **draft PR** per the repo's PR policy and merge once review is
green. Don't merge this PR without step 2 complete — goreleaser's template
evaluator will fail hard on a missing `HOMEBREW_TAP_TOKEN` env var and brick
the next release.

### Step 4 — cut the next release and watch

```sh
gh run watch --repo CoreyRDean/yoink-n-yeet $(gh run list --repo CoreyRDean/yoink-n-yeet --workflow=release.yml --limit 1 --json databaseId --jq '.[0].databaseId')
```

On success you should see a new commit land on `CoreyRDean/homebrew-tap`
main authored by `goreleaser-bot <bot@yoinknyeet.dev>` bumping the formula.
Delete this runbook's *Manual bump runbook* section once that's confirmed
working for two consecutive releases.

---

## Troubleshooting

- **`gh api` PUT returns 422 "sha wasn't supplied"** — the file already
  exists. Fetch its SHA with a GET first and include `-f sha=<sha>` on the
  PUT. This bit us on v0.1.3.
- **Formula class name mismatch** — the filename must be
  `yoink-n-yeet.rb` and the class must be `YoinkNYeet` (each hyphen-separated
  word capitalized, hyphens dropped). Homebrew computes one from the other
  and will refuse to load a formula that disagrees.
- **`brew install` downloads then fails with `SHA256 mismatch`** — the
  checksums file is canonical. If the formula disagrees, the formula is
  wrong; re-pull checksums and re-edit. Never "fix" this by recomputing
  against a downloaded tarball without reconciling against the signed
  checksums file.
- **Archive layout drift** — the formula's `bin.install "yoink-n-yeet"`
  assumes the binary is at the root of the tarball. Goreleaser's archive
  stanza in `.goreleaser.yaml` currently puts it there; if that ever
  changes (e.g. adding a `wrap_in_directory:` option), the formula's
  `install` block has to change with it.

---

## Appendix: v0.1.3 checksums (for reference)

Pulled from
`yoink-n-yeet_0.1.3_checksums.txt` on 2026-04-20:

- `darwin_amd64`: `0717bd7d773b3f7e32d968d83a850fc0ecbf94f5798f3bc8d89f8cca329a69fd`
- `darwin_arm64`: `de79e432bf220571b8f4aa0f60208d358339db4e3a7b00f74716178b36877684`
- `linux_amd64` : `e72ce153e5361f542332fe8c00fdef6f697944fbfb3ec78f6d8fef0833d2a514`
- `linux_arm64` : `c2fcff316fb50a319d847b0d73a4b06fe8636c5cc78b0edda76615f9fc36e494`
