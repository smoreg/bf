# Contributing

Thanks for considering a contribution to `bf`. This guide covers the basics.

## Local setup

```sh
git clone https://github.com/smoreg/bf
cd bf
make test     # runs go test -race
make lint     # runs golangci-lint
make cover    # produces coverage.out and prints summary
```

You need Go 1.26+ and (for `make lint`) a recent
[golangci-lint](https://golangci-lint.run/usage/install/).

## Pull-request checklist

- New behaviour comes with tests (`make test` stays green with `-race`).
- `make lint` is clean.
- Update `CHANGELOG.md` under the `## [Unreleased]` section for any
  user-visible change.
- Public API changes carry godoc comments on the new symbols.

## Commit style

Conventional-commit prefixes are appreciated but not enforced:

```
feat: add WithRetry option
fix: stop cleaner goroutine on Stop
docs: clarify layer lifetime
```

Keep the subject under ~70 characters.

## Reporting bugs

Use the [bug report template](.github/ISSUE_TEMPLATE/bug_report.md) and
include a minimal reproduction and your Go version.

## Security

Please do not file public issues for security-sensitive bugs. See
[SECURITY.md](SECURITY.md) for the disclosure process.
