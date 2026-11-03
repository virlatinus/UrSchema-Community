# Security Policy

## Supported versions

Security fixes go into the latest release. Please upgrade before reporting, and include the version (release tag or commit) you tested.

## Reporting a vulnerability

Please **do not open a public issue** for security problems.

Report them privately through GitHub: go to the repository's **Security** tab and choose **Report a vulnerability** ([direct link](https://github.com/virlatinus/UrSchema-Community/security/advisories/new)).

Include what you can of:

- the affected component (backend API, frontend, Docker image, release tarball)
- steps to reproduce, or a proof of concept
- the impact you expect (for example: reading saved connection passwords, bypassing login or read-only mode)

You should get an acknowledgement within a few days. We'll keep you updated while we work on a fix, and credit you in the release notes unless you'd rather stay anonymous.

## Scope

UrSchema is a database administration tool: anyone who can use it has the access of the database accounts in its saved connections. These are expected behavior rather than vulnerabilities:

- full database access for a logged-in user, or for anyone reaching an instance started without `URSCHEMA_PASSWORD`
- plain-HTTP traffic when TLS isn't configured
- side effects of database functions that are allowed inside a read-only transaction (read-only mode is a guardrail, see the README)

Ways to get around the login, read-only mode, the loopback-only default, or the encryption of saved connections are in scope.

See the [Security section of the README](README.md#-security) for deployment guidance.
