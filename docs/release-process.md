# Release process: Conventional Commits + svu + an rc channel

This is the design for how para gets versioned and released from here forward.
It answers four questions: what a commit message must look like, how a branch
gets from a merged PR to a published binary, what enforces the first
question, and where the machinery lives. It builds on top of, and changes
very little about, the release engineering `.goreleaser.yaml` and
`.github/workflows/release.yml` already established in Phase 15 — this is the
layer that decides *what tag to push*; goreleaser still decides what happens
once one lands.

Decisions behind this design, made by the repo owner 2026-08-08:

- Version computation is Go-native (`svu`), not the JS `semantic-release`
  toolchain this repo briefly carried before the "clean house" commit
  (`8d4a3a0`) removed it. No Node/npm anywhere in the release path.
- `rc` is a **feed-forward staging branch**: feature work lands there first,
  publishes `-rc.N` prereleases, and periodically graduates into `main` as a
  real release. It is not an independent version line.
- `main` is brought current with `impl` (the completed Go rewrite) before any
  of this goes live; `impl` keeps serving as the day-to-day integration branch
  feeding `rc`.
- Every individual commit must be a valid Conventional Commit — not just PR
  titles — so PRs merge with **"Create a merge commit"**, never squash or
  rebase-merge, and both a local hook and CI check every commit.
- First tag is `v0.1.0`, placed by hand on the commit where `main` picks up
  the rewrite. `svu` only ever looks at commits reachable after it, so the
  untagged, non-conventional history before this point is inert.
- Release notes live on GitHub Releases only (goreleaser already generates
  them from the git log); no committed `CHANGELOG.md`, no write-back commits.

## 1. Commit message format

[Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/):

```
<type>[optional scope][!]: <description>

[optional body]

[optional footer(s)]
```

Allowed `type`s (matches what `.goreleaser.yaml`'s changelog filter and the
existing commit vocabulary on `impl` already assume):

| type       | release effect                          |
|------------|------------------------------------------|
| `feat`     | minor bump                                |
| `fix`      | patch bump                                |
| `perf`     | patch bump                                |
| `revert`   | patch bump                                |
| `docs`, `test`, `chore`, `ci`, `refactor`, `style`, `build` | no bump — excluded from release notes |

Breaking change: `!` after the type/scope (`feat!:` or `fix(tree)!:`), or a
`BREAKING CHANGE:` footer. Either forces a major bump regardless of type.

Merge commits (`Merge pull request #N from ...`, `Merge branch 'rc' into
main`) are exempt — GitHub writes those, not a contributor, and they carry no
release-relevant type of their own.

## 2. Branch model

```
feature/fix branches ──PR (merge commit)──▶ rc ──publishes v X.Y.Z-rc.N
                                              │
                                              │ PR (merge commit), when ready
                                              ▼
                                             main ──publishes v X.Y.Z
```

- Feature and fix branches are short-lived, branched from `rc`, PR'd back
  into `rc`.
- Every merge into `rc` runs the version workflow (§4), which may cut a new
  `-rc.N` prerelease if the merged commits included a `feat`/`fix`/etc.
- When `rc` is ready to ship, it's PR'd into `main` (merge commit, so `main`
  gets the same commits `rc` already published prereleases from — svu ignores
  prerelease tags when computing `main`'s next *stable* version, so this
  naturally "graduates" `v1.3.0-rc.2` to a clean `v1.3.0`).
- After promotion, `rc` is reset to `main`'s tip (`git push --force-with-lease
  origin main:rc`) so the next round of features starts from a clean base and
  the `-rc.N` counter restarts at `.1` for whatever version comes next.
- **Hotfixes** may PR directly into `main`, skipping `rc`, for anything
  urgent enough not to wait for the staging cycle. The version workflow reacts
  the same way regardless of which branch it's watching.

## 3. Enforcement

One script, two callers, so there's exactly one definition of "valid":

**`scripts/check-commit-msg.sh`** — reads a commit message (as a file path,
matching git's `commit-msg` hook contract), rejects it (non-zero exit, reason
on stderr) unless it matches the Conventional Commits grammar from §1, and
exempts `Merge ` subject lines.

**Local hook** — `make install-hooks` sets
`git config core.hooksPath .githooks`, and `.githooks/commit-msg` calls the
script. Opt-in per clone (git doesn't auto-install hooks), documented in
README's contributing section. Catches a malformed message before it's even
committed, not just before it's pushed.

**CI** — a job in `.github/workflows/ci.yml` (`commit-messages`, running
alongside the existing `test`/`lint`/`install` jobs) that, on every PR,
walks `git log --format=%s origin/<base>..HEAD` and runs the same script over
each line. This is the real gate — the local hook is a courtesy, not a
guarantee, since it's opt-in and bypassable with `--no-verify`.

## 4. Version workflow

New: `.github/workflows/version.yml`.

```yaml
on:
  push:
    branches: [main, rc]

jobs:
  version:
    if: github.repository == 'colchuck-ai/para'   # never fires from a fork
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with: { go-version: "1.25" }

      # Same two gates release.yml already requires of a tagged commit — a
      # broken commit on main/rc must not become a tag just because it landed
      # before CI's own report came back.
      - run: make lint
      - run: make test

      - name: compute next version
        id: v
        run: |
          go install github.com/caarlos0/svu/v3@latest
          current=$(svu current)
          if [ "${{ github.ref_name }}" = "rc" ]; then
            next=$(svu prerelease --prerelease rc)
          else
            next=$(svu next)
          fi
          echo "current=$current" >> "$GITHUB_OUTPUT"
          echo "next=$next" >> "$GITHUB_OUTPUT"

      - name: tag and push
        if: steps.v.outputs.next != steps.v.outputs.current
        run: |
          git tag "${{ steps.v.outputs.next }}"
          git push origin "${{ steps.v.outputs.next }}"
```

Pushing the tag is the entire hand-off — the existing tag-triggered
`release.yml` picks it up unchanged and runs goreleaser exactly as it does
today. Two small additions to that side, though:

- **`.goreleaser.yaml`**: add `release: { prerelease: auto }` so a
  `-rc.N` tag publishes as a GitHub prerelease and a plain tag doesn't.
  `scripts/install.sh`'s "latest" URL is unaffected — GitHub's
  `releases/latest` already excludes prereleases, so an `-rc` build can never
  be resolved as latest.
- **`.goreleaser.yaml`**'s `changelog:` block can optionally group entries by
  type (`Features`, `Bug fixes`, …) now that every commit has one — a
  cosmetic upgrade to the release notes, not load-bearing.

## 5. Repo settings

- Disable **squash merge** and **rebase merge**, leaving only "Create a merge
  commit" — makes it impossible to accidentally collapse a PR's individual
  conventional commits into one, or rewrite their SHAs.
- Branch protection on `main` and `rc`: require the `test`, `lint`, and new
  `commit-messages` checks before merge; no force-push, no direct push
  (except the version workflow's own tag push, which targets a tag ref, not
  the branch).

## 6. Bootstrap sequence (one-time)

1. Merge/fast-forward `impl` into `main`.
2. Tag that commit `v0.1.0` by hand and push it — the one tag in this scheme
   that doesn't come from `svu`.
3. Create `rc` from `main` at that same commit.
4. Add `scripts/check-commit-msg.sh`, `.githooks/commit-msg`, the
   `commit-messages` CI job, and `.github/workflows/version.yml`.
5. Update `.goreleaser.yaml` with `release.prerelease: auto`.
6. Flip the repo merge-button settings and add branch protection.
7. From here on: feature branches → `rc` → `main`, as in §2.

## Explicitly out of scope (for now)

- A committed `CHANGELOG.md` — GitHub Releases carries the notes; revisit if
  an in-repo, browsable-without-GitHub history is wanted later.
- Any JS/Node tooling (`semantic-release`, `commitlint`, `husky`) — the
  Go-native `svu` + hand-rolled shell script covers the same ground without
  reintroducing a second language toolchain to a single-language repo.
