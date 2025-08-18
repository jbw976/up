# Contributing

For styling guidelines, see [this document](https://github.com/crossplane/crossplane/blob/master/contributing/README.md).

## Development Environment

The project includes a git submodule that includes various helpers. After
cloning, you'll need to make `git` download it with the following command.

```bash
make submodules
```

The rest is just a usual Golang CLI project where you can find the executables
under `cmd` folder.

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
  embedded into the docs site at that level. We render all heading levels the same in the terminal
  the same in the terminal, so it's generally best to use only level 4.
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

This is a slimmed-down version of the release process described [here](https://github.com/crossplane/release).

1. **feature freeze**: Merge all completed features into main development branch
   of all repos to begin "feature freeze" period.
1. **branch repo**: Create a new release branch using the GitHub UI for the
   repo (e.g. `release-0.25`).
1. **tag release**: Run the `Tag` action on the _release branch_ with the
   desired version (e.g. `v0.25.0`).
1. **build/publish**: Run the `CI` action on the tag.
1. **tag next pre-release**: Run the `tag` action on the main development branch
   with the `-0.rc.0` for the next release (e.g. `v0.26.0-0.rc.0`). (**NOTE**:
   we added the `-0.` prefix to allow correctly sorting release candidates)
1. **verify**: Verify all artifacts have been published successfully, perform
   sanity testing.
   - Check in https://cli.upbound.io/stable?prefix=build/release-0.25/v0.25.0.
     Download some binaries / package formats and smoke test them, e.g. by
     - (all platforms) download your architecture from the `bin` folder and run
       it: `up version`.
     - TODO: add more here
   - **note**: You may keep downloading the old version for a while until CDN
     cache is refreshed.
1. **promote**: Run the `Promote` action on the release branch with the release
   version being the tag name (e.g. `v0.25.0`) and the channel being
   `alpha` or `stable`.
1. **verify promotion**: Check that https://cli.upbound.io/stable?prefix=stable/v0.25.0/
   has the new version.
1. **update homebrew**: Run [`Bump Formula`](https://github.com/upbound/homebrew-tap/actions/workflows/bump-formula.yaml) action to open a PR in Homebrew
   for the new version. Get approval and merge.
1. **release notes**:
   - Open the new release tag in https://github.com/upbound/up/tags and click "Create
     release from tag".
   - "Generate release notes" from previous release ("auto" might not work).
   - Make sure the release notes are complete, presize and well formatted.
   - Publish the well authored Github release.
1. **invalidate CDN cache**: If needed see the internal Notion documentation.
1. **wait for CDN**: Wait for CloudFront to distribute the artifacts, e.g. wait
   until `curl -sL https://cli.upbound.io | sh -x && ./up version` gives the new
   release.
1. **announce**: Announce the release on Twitter, Slack, etc.
   - Crossplane Slack #Upbound: https://crossplane.slack.com/archives/C01TRKD4623
   - TODO: where else?
