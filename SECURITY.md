# Security Policy

## Supported versions

Until the project reaches v1.0.0, only the latest stable release receives
security fixes. Nightly builds receive fixes as they land on `main`.

## Reporting a vulnerability

**Please do not file public issues for security bugs.**

Report privately via one of:

- GitHub's [private vulnerability reporting](https://github.com/CoreyRDean/yoink-n-yeet/security/advisories/new)
- Email: `coreyrdean` on GitHub → the email listed on his GitHub profile

Include:

- A clear description of the vulnerability and the impact
- Steps to reproduce or a proof-of-concept
- The version(s) affected (`yk --version` output helps)
- Any suggested remediation

You can expect:

- Acknowledgement within 72 hours
- A coordinated disclosure timeline, typically within 30 days
- Credit in the advisory and CHANGELOG unless you'd prefer anonymity

## Known security-relevant surfaces

- **Clipboard contents frequently hold secrets.** Treat stack entries as sensitive.
- **`--update` fetches and replaces the binary.** Every update path verifies
  sha256 checksums against the signed release manifest; cosign keyless
  signatures are verified when available.
- **`install.sh` via `curl | bash`.** The installer is pinned to this
  repository and verifies downloaded artifacts. Read it before piping to
  shell if you prefer.
- **No telemetry.** The only network I/O is the opt-in update check against
  `api.github.com/repos/CoreyRDean/yoink-n-yeet/releases`.
