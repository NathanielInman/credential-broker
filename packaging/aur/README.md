# AUR packages

`credential-broker-bin` repackages the Linux release archives. After a release:

1. Set `pkgver` (and `pkgrel=1`) in `PKGBUILD`, and replace both `sha256sums_*` with the lines for the linux archives in the release's `SHA256SUMS`.
2. `makepkg -f` to check it builds, then `makepkg --printsrcinfo > .SRCINFO`.
3. Copy `PKGBUILD` and `.SRCINFO` into a clone of `ssh://aur@aur.archlinux.org/credential-broker-bin.git` and push.
