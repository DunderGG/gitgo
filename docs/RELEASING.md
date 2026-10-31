# Releasing GitGo

The [release workflow](../.github/workflows/release.yml) builds and publishes each release. You write the notes and push a tag.

## Publish a release

1. Add a section for the version to [CHANGELOG.md](../CHANGELOG.md), such as `## [1.2.0] - 2026-11-15`, then commit and push it. The workflow finds the section by the version in brackets.
2. Tag that commit and push the tag:

   ```bash
   git tag -a v1.2.0 -m "GitGo 1.2.0"
   git push origin v1.2.0
   ```

A tag with a suffix, such as `v1.2.0-beta.1`, makes a pre-release.

## What the workflow does

1. Builds on a Windows, a macOS and a Linux runner, with the tag's version stamped into the app (`X.Y.Z` in `wails.json`, which sets the `.exe` version info, the `Info.plist` and the installer):
   - **Windows:** NSIS installer and portable `.exe`
   - **macOS:** universal `.dmg` for Intel and Apple Silicon, signed ad hoc
   - **Linux:** `.deb` and `.tar.gz`, built on Ubuntu 22.04 so they run on Ubuntu 22.04+ and Debian 12+
2. When all three builds succeed, creates the GitHub Release for the tag with the packages and a `SHA256SUMS` file. The notes are the version's CHANGELOG.md section, then the guide to the downloads in [.github/release-downloads.md](../.github/release-downloads.md), then GitHub's link to the commits since the previous release. Without a CHANGELOG.md section, the run warns and the notes only describe the downloads.

If any build fails, no release is made.

## Try the packaging first

Run the workflow by hand: **Actions → Release → Run workflow** on `main`. It builds the same packages, versioned from `wails.json` with `-dev.<commit>` added, and keeps them as artifacts of the run (`packages-Windows`, `packages-macOS`, `packages-Linux`), without publishing anything.

## Fix a release

- **Only the publish job failed:** click **Re-run failed jobs** on the run. It creates the release, or uploads to it if it already exists.
- **Wrong notes:** fix CHANGELOG.md, and edit the notes on the release page (**Edit**). The workflow never changes the notes of a release that already exists.
- **A build failed:** no release was made. Fix the problem on `main`, then move the tag to the fixed commit:

  ```bash
  git push origin --delete v1.2.0
  git tag -d v1.2.0
  git tag -a v1.2.0 -m "GitGo 1.2.0"
  git push origin v1.2.0
  ```

- **A published release needs rebuilding:** delete it first with `gh release delete v1.2.0 --yes` (or on the release page), then move the tag as above. Deleting a release keeps its tag, so the tag must still be deleted, or pushing the new one is rejected. Only do this before anyone has downloaded the release; after that, publish a new version instead.
