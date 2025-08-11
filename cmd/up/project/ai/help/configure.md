The `configure-tools` command generates configuration to make commonly-used AI
development tools more effective at working in an Upbound project.

#### Supported Tools

* Claude Code
* Cursor
* Gemini CLI

#### Examples

Create `GEMINI.md` and `.gemini/settings.json`:

```shell
up project ai configure-tools --gemini-cli
```

Create `CLAUDE.md`, `.claude/settings.json`, and `.mcp.json`:

```shell
up project ai configure-tools --claude-code
```

Create `.cursor`:

```shell
up project ai configure-tools --cursor
```

Create configuration for all three tools:

```shell
up project ai configure-tools --gemini-cli --claude-code --cursor
```
