# Releasing

Releases are automated. Merging to `main` builds the image, pushes it to Docker
Hub, tags the commit, and cuts a GitHub Release - no manual version bump, no
local `docker build`. The workflow is `.github/workflows/release.yml`.

## How the version is chosen

The next version is computed from the last git tag plus the Conventional Commit
prefixes of everything merged since it:

| Commit(s) since last tag                     | Bump  | Example         |
| -------------------------------------------- | ----- | --------------- |
| `feat!:` / `fix!:` / a `BREAKING CHANGE:` note | major | 1.2.3 -> 2.0.0  |
| `feat:`                                       | minor | 1.2.3 -> 1.3.0  |
| `fix:` / `perf:`                              | patch | 1.2.3 -> 1.2.4  |
| only `chore:` / `docs:` / etc.                | none  | no release      |

Numbers are plain integers - ten fixes in a row is `1.2.13`, not `1.3.0`. A
minor only happens when a feature lands and a major only when you mark a commit
breaking, so the version always reflects compatibility, not a release count.

Highest bump wins: a push containing both a `fix:` and a `feat:` releases a
minor.

## Forcing a specific bump

Actions -> **release** -> **Run workflow** -> pick `patch` / `minor` / `major`.
Use this to force a major without a `!` commit, or to release when the automatic
rule would skip.

## One-time setup

1. **Secrets** (Settings -> Secrets and variables -> Actions):
   - `DOCKERHUB_USERNAME` = `mvsrinath`
   - `DOCKERHUB_TOKEN` = a Docker Hub **access token** with Read & Write
     (Account Settings -> Security -> New Access Token). Use a token, not your
     password.
2. **Seed tag** so CI has a version to count from:
   ```
   git tag -a v1.2.2 -m "Release 1.2.2" b9d18fb
   git push origin v1.2.2
   ```
3. **Settings -> Actions -> General -> Workflow permissions**: "Read and write
   permissions", so CI can push the tag and publish the Release.

## Manual fallback

`make release` still works for a build from your own machine (it reads `VERSION`
from `make/00-config.mk`). It is only a fallback; if you use it, bump that
`VERSION` by hand first.
