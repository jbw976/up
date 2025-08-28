# Contributing

## Style

For the most part, we follow the [Crossplane coding
style](https://github.com/crossplane/crossplane/blob/main/contributing/README.md#coding-style),
like most other Upbound codebases. Exceptions are described below.

### Test Assertions

Unlike Crossplane, we use an assertion library, specifically
[`gotest.tools/v3/assert`](https://pkg.go.dev/gotest.tools/v3/assert). We prefer
this assertion library because it offers a limited number of assertions, each of
which is relatively simple and self-explanatory (compared to, e.g., the
`testify` libraries, which offer a huge number of complex assertions).

It's also fine to use the go `testing` package and `cmp` (as in Crossplane), but
`assert` often makes tests easier to read and write.

## Development Environment

This project uses [goreleaser](https://goreleaser.com/) for builds and
releases. Makefile targets are provided for convenience.

We use [golangci-lint](https://golangci-lint.run/) to ensure code consistency
and quality. Older files may have many linter issues; please fix them when you
need to touch the file. In new code, please use `nolint` directives judiciously,
preferring to fix lint issues rather than ignore them unless they're unavoidable
or the fix is clearly less readable.

Otherwise, we use only the standard `go` toolchain, and the project structure
follows standard Go practices.

## kong

We use [kong](https://pkg.go.dev/github.com/alecthomas/kong) as our CLI
framework. Each command is defined as a struct, in which fields become
subcommands, positional arguments, or flags. Kong's [struct
tags](https://pkg.go.dev/github.com/alecthomas/kong#readme-supported-tags) can
be used to control many behaviors, including validation, auto-completion, and
documentation.

## Embedded Documentation

The CLI is self-documenting. Short descriptions for commands and flags should be
added using kong struct tags. Longer help should be returned by each command's
`Help()` method. This help can be formatted using markdown, which we render in
the console using the
[glamour](https://pkg.go.dev/github.com/charmbracelet/glamour) library.

The `up generate-docs` command is used to generate CLI reference documentation
for [docs.upbound.io](https://docs.upbound.io/reference/cli-reference). Markdown
returned by each command's `Help()` is embedded verbatim into this
documentation; making the generated docs look good requires a touch of care:

* Don't use headings below level 4 (i.e., `#### Section`), since our help gets
  embedded into the docs site at that level. We render all heading levels the
  same in the terminal, so it's generally best to use only level 4.
* Don't use `<` characters in code blocks. We have to replace the `<` character
  with `&lt;` when we generate markdown for the documentation website, but the
  escaped version ends up being displayed verbatim in code blocks. `<` is fine
  in inline code (backtick expressions), and all other special characters are
  fine in code blocks.

If in doubt about whether your help will look good, you can easily check your
work by cloning the [upbound/docs](https://github.com/upbound/docs) repo,
running `make start`, then using `up generate-docs` to update the CLI reference
section.

## Telemetry

The CLI sends anonymous product telemetry data to Upbound via OpenTelemetry. A
tracing span is emitted for each command and includes basic information such as
the CLI version, the command's name, which flags were set (but not their values,
since they may be sensitive), and whether the user is logged in (but not their
login identity - again, sensitive).

Individual commands can add more data to the span in two ways:

1. Flag values we deem non-sensitive and interesting can be included by adding
   the struct tag `telemetry:"true"`. Remember that we must not send identifying
   or sensitive data as telemetry, so think carefully about the nature of the
   flag value before adding it.
2. Arbitrary values can be added by having a command's `Run` or `AfterApply`
   methods take a `trace.Span` argument and using its `SetAttributes` method.

### Testing

When working on telemetry, it can be useful to see the telemetry data that will
be sent. You can do this by running a local OpenTelemetry collector:

1. Create a file called `otel-config.yaml` with the following contents:

   ```yaml
   receivers:
     otlp:
       protocols:
         grpc:
           endpoint: 0.0.0.0:4317
         http:
           endpoint: 0.0.0.0:4318
           cors:
             allowed_origins:
               - "*"

   exporters:
     debug:
       verbosity: detailed

   service:
     pipelines:
       traces:
         receivers: [otlp]
         processors: []
         exporters: [debug]
   ```

2. Start the OpenTelemetry collector in a container:

   ```console
   docker run -it --rm \
       -v otel-config.yaml:/otel-config.yaml \
       -p 4318:4318 -p 4317:4317 \
       otel/opentelemetry-collector --config otel-config.yaml
   ```

3. Configure the CLI to use the local collector:

    ```console
    up config set telemetry.endpoint http://localhost:4317
    up config set telemetry.insecure true
    ```

Now when you run `up`, you should see spans emitted on the terminal where you're
running the collector.

## Release Process

The release process for `up` is semi-automated through GitHub Actions. The
following are the manual steps involved.

1. **branch repo**: For the first release of a particular minor version, create
   a new release branch using the GitHub UI for the repo (e.g. `release-0.25`).
1. **tag release**: Run the `Tag` action on the _release branch_ with the
   desired version (e.g. `v0.25.0`). This triggers the Release workflow in
   GitHub Actions, which will do a build, create a GH release, and upload
   artifacts.
1. **tag pre-release**: Run the `Tag` action on _main_ to create an `rc.0`
   version of the _next_ minor release (e.g. `v0.26.0-0.rc.0`). This will not
   trigger a release, and ensures subsequent builds from main have sensible
   version numbers.
1. **verify**: Verify all artifacts have been published successfully, perform
   sanity testing.
   * Navigate to
     [build/\<version>/\<version>/bin](https://cli.upbound.io/_?prefix=build/v0.40.1/v0.40.1/bin)
     in the S3 bucket listing.
   * Check that all platforms are present and have all the binaries.
1. **promote**: Run the `Promote` action on the _tag_ with the release version
   being the tag name (e.g. `v0.40.1`) and the channel being `alpha` or
   `stable`.
   * Promote RC releases to `alpha`.
   * Promote regular patch releases to both `alpha` and `stable`.
   * Don't promote patch releases for older minor versions.
1. **verify promotion**: Check that
   [stable/current](https://cli.upbound.io/_?prefix=stable/current/) has the new
   version.
1. **update docs**: Download the new release, check out the [docs
   repo](https://github.com/upbound/docs/), and run `up generate-docs
   --output-dir=<docs path>` to generate CLI reference docs. Create a PR in the
   docs repo with the update.
1. **update homebrew**: Run [`Bump
   Formula`](https://github.com/upbound/homebrew-tap/actions/workflows/bump-formula.yaml)
   action to open a PR in Homebrew for the new version. Get approval and merge.
1. **release notes**:
   * Open the new release tag in the [GitHub
     UI](https://github.com/upbound/up/tags) and click "Create release from
     tag".
   * "Generate release notes" from previous release ("auto" might not work).
   * Make sure the release notes are complete, presize and well formatted.
   * Publish the well authored Github release.
1. **invalidate CDN cache**: If needed see the internal Notion documentation.
1. **wait for CDN**: Wait for CloudFront to distribute the artifacts, e.g. wait
   until `curl -sL https://cli.upbound.io | sh -x && ./up version` gives the new
   release.
1. **announce**: Announce the release on Slack.
   * Upbound Slack [#shiproom](https://upboundio.slack.com/archives/C08G69UGLJG)
   * Crossplane Slack [#upbound](https://crossplane.slack.com/archives/C01TRKD4623)
