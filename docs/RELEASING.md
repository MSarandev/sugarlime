# Releasing

sugarlime is a single static binary with no runtime files: the settings registry
and the logo are compiled in, and `CGO_ENABLED=0` means nothing links against
the host's libc. So a release is just "build four binaries and publish them".

[GoReleaser](https://goreleaser.com) does that from `.goreleaser.yaml`, and
`.github/workflows/release.yml` runs it when a `v*` tag is pushed.

## Cutting a release

```sh
# 1. make sure main is green and the changelog is honest
make test
$EDITOR CHANGELOG.md     # move Unreleased into a version heading

git commit -am "Release v0.1.0"

# 2. tag and push
git tag -a v0.1.0 -m "v0.1.0"
git push origin main v0.1.0
```

The workflow then runs the test suite, checks the generated registry is current,
and publishes a GitHub Release with:

| Artifact | For |
| :- | :- |
| `sugarlime_0.1.0_darwin_arm64.tar.gz` | Apple silicon |
| `sugarlime_0.1.0_darwin_amd64.tar.gz` | Intel Macs |
| `sugarlime_0.1.0_linux_arm64.tar.gz` | Linux on ARM |
| `sugarlime_0.1.0_linux_amd64.tar.gz` | Linux on x86-64 |
| `.deb`, `.rpm`, `.apk` | Linux package managers |
| `checksums.txt` | verifying any of the above |

A tag like `v0.2.0-rc1` publishes as a pre-release automatically.

## Trying it without publishing

```sh
brew install goreleaser     # or see goreleaser.com/install
make release-check          # validates .goreleaser.yaml
make snapshot               # builds everything into ./dist, publishes nothing
```

Opening a pull request that touches `.goreleaser.yaml` or the release workflow
also runs a snapshot build in CI and attaches the archives, so a packaging
mistake surfaces before a tag exists.

## Adding a Homebrew tap

`brew install` is the nicest way to install this on macOS, and it works on Linux
too. It needs one-time setup, because a tap lives in its own repository:

1. Create a public repo named **`homebrew-tap`** under the same owner. The name
   matters: it is what makes `brew install MSarandev/tap/sugarlime` work.
2. Create a fine-grained personal access token with **Contents: write** on that
   repo only, and add it to this repo as the secret `HOMEBREW_TAP_TOKEN`.
3. Add this to `.goreleaser.yaml`:

   ```yaml
   brews:
     - name: sugarlime
       repository:
         owner: MSarandev
         name: homebrew-tap
         token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
       homepage: https://github.com/MSarandev/sugarlime
       description: A terminal UI for Claude Code's settings.json.
       license: MIT
       test: |
         system "#{bin}/sugarlime", "-version"
   ```

4. Pass the token to the release step in `.github/workflows/release.yml`:

   ```yaml
           env:
             GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
             HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
   ```

5. Add the install instructions to the README and to `release.footer` in
   `.goreleaser.yaml`. They are deliberately left out until the tap exists, so
   the README never advertises an install path that fails.

GoReleaser then commits an updated formula to the tap on every release.

## macOS and Gatekeeper

The binaries are not signed or notarized, which is normal for a developer CLI
and fine for the ways people will install it:

- **Homebrew** and **`go install`** are not quarantined, so they just run.
- A tarball fetched with `curl` or `wget` is not quarantined either.
- A tarball downloaded in a **browser** is. macOS will refuse to run it until
  the flag is cleared:

  ```sh
  xattr -d com.apple.quarantine ./sugarlime
  ```

The README says this, so nobody has to work it out from an unhelpful dialog.

If you later want signed and notarized macOS binaries, it needs an Apple
Developer account ($99/year), a Developer ID certificate in CI, and
`notarize:` configuration in GoReleaser. It is worth doing only if you expect
people to install from a browser download.

## What a release does not include

- **Windows.** The code assumes POSIX paths in places and nothing is tested
  there. `managedPath` has a Windows branch, but that is as far as it goes.
- **A `curl | sh` installer.** Tarballs and Homebrew cover the same ground
  without asking anyone to pipe a script into a shell.
