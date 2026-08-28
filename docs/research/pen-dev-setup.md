# pen.dev: what is installed, what an agent can drive, and what a `.pen` file is

Resolves [#408](https://github.com/markgoho/doula-cloud/issues/408), a task ticket of the wayfinder map
[Holistic application design](https://github.com/markgoho/doula-cloud/issues/405). Everything below was
observed on this machine on 2026-08-28, not read from documentation. Where a claim comes from docs
rather than observation, it says so.

## What is installed

| | |
|---|---|
| Desktop app | `/Applications/Pen.app`, Electron, bundle id `dev.pencil.desktop`, **v1.2.7**, URL scheme `pencil://` |
| CLI | `@pen.dev/cli@0.3.5`, a **project devDependency**, binaries `pen` and `pencil` in `node_modules/.bin` |
| MCP server | `mcp-server-darwin-arm64`, a 4.7 MB native binary inside the app bundle |
| Account | `markgoho@gmail.com`, workspace `Personal (markgoho)`, status Active |
| Cost | Free. pen.dev's pricing page states *"pen.dev is currently free"* — no tier, no waitlist |

### bun, not npm

The docs say `npm install -g @pen.dev/cli`. **A project-local bun install works and is what we use**, so
no npm exception is needed and the repo-wide bun rule holds. Run it as `bunx pen`. A global install would
have put an unpinned tool outside the lockfile for no benefit.

### Authentication

`pen login` is interactive (email + password or OTP) and writes `~/.pencil/session-cli.json`. The desktop
app keeps a separate `~/.pencil/session-desktop.json`; **logging into the desktop does not log in the
CLI**, which is why the CLI showed no session despite the app being signed in since 2026-08-10.
`PEN_CLI_KEY` is the non-interactive alternative for CI, via organisation Developer Keys.

## Enabling the MCP server in Claude Code

**This machine runs with `CLAUDE_CONFIG_DIR=/Users/mgoho/.claude-personal`**, so the live config is
`~/.claude-personal/.claude.json`. That single fact explains the whole problem, and it is easy to get
wrong — this ticket got it wrong once before catching it.

Pen writes itself into `~/.claude.json` on install — the *default* config path:

```jsonc
// ~/.claude.json  <- NOT the config this machine reads
"mcpServers": {
  "pencil": {
    "command": "/Applications/Pen.app/Contents/Resources/app.asar.unpacked/out/mcp-server-darwin-arm64",
    "args": ["--app", "desktop", "--agent", "claudeCodeCLI"],
    "env": {}, "type": "stdio"
  }
}
```

The live config's global `mcpServers` is **empty**. So pencil was not disabled here, and not
mis-scoped — it was simply **absent**, because Pen's installer wrote to a config directory this machine
does not use. Anything reading `~/.claude.json` to reason about MCP state on this machine is reading a
stale file.

**Do not hand-edit either JSON file.** Use the CLI, which writes to whichever config is live:

```bash
claude mcp add pencil --scope local -- \
  /Applications/Pen.app/Contents/Resources/app.asar.unpacked/out/mcp-server-darwin-arm64 \
  --app desktop --agent claudeCodeCLI
```

`claude mcp get pencil` then reports **`Status: ✔ Connected`**. `--scope local` keeps it private to this
project, which is right while pen.dev is doula-cloud work; `--scope user` would widen it to every repo.
Remove with `claude mcp remove pencil -s local`. A session restart is still needed before the tools
appear in a running session.

Two diagnostic notes. `claude mcp list` and `claude mcp get` read the **live** config, so they are the
authority — a server present in a JSON file but absent from `claude mcp get` is not registered.
And the server binary takes `-app` (*"pen.dev app to connect to"*), `-agent`, `-conversation_id`, and
`-enable_spawn_agents`, which exposes a `spawn_agents` tool and is off by default.

## What the MCP server exposes

**The surface moves. It changed underneath this ticket.** The app self-updated from v1.2.4 to v1.2.7
mid-session, and the tool list changed with it. Both were captured by an `initialize` + `tools/list`
handshake straight against the binary (server `pencil` v1.0.0 — an internal version that did **not**
change — protocol `2025-06-18`, capabilities `logging` and `tools`).

**v1.2.7, current:**

| Tool | What it does |
|---|---|
| `execute` | The workhorse — mutates the canvas. Self-describing via `get_app_state` |
| `get_app_state` | App state, current user selection, "essential information to get started on a task" |
| `browser` | Loads a real URL in an integrated browser and can **reproduce the page as editable canvas layers** |
| `get_style` | Lists and loads **visual style archetypes** — configurable fonts, colours, imagery. Reference values only; styles do not save variables |
| `read_skill` | Reads pen.dev's own `SKILL.md` teaching an agent how to design on the canvas, plus referenced files like `execute.md`, `guide/web-app.md` |

**v1.2.4 also had** `get_guidelines` (now split into `get_style` + `read_skill`), `get_screenshot`,
`export_html` (HTML + Tailwind or HTML + CSS) and `export_nodes` (PNG/JPEG/WEBP/PDF).

Two consequences.

**There is now no export tool on the MCP surface at all.** Not for HTML, not for images. So the question
"does pen.dev export Svelte" is the wrong question in a second way — it no longer exports anything
through MCP. Code has to come from an agent reading canvas state and writing files itself. The CLI keeps
image export as a flag (`--export`, `--export-type`), which is how this ticket's smoke test produced a PNG.

**`browser` is a stronger Code → Design path than the docs advertise**, and it survived the update
unchanged. The docs describe recreating a component from a source file. The tool actually loads a
*running* page and imports it as editable layers, and each imported node's `context` field carries the
source element's tag **prefixed with its detected code component name** — the description's own example
is `"Card - div"`. Pointed at a local dev server, this pulls real application screens onto the canvas
with component attribution rather than re-deriving them from source.

`get_style` could not be called from a headless handshake — it returned nothing without the app
connected. Listing the style archetypes is left to [#411](https://github.com/markgoho/doula-cloud/issues/411),
where they matter: an opinionated set of named visual archetypes is a fourth source of candidate
directions for [#409](https://github.com/markgoho/doula-cloud/issues/409)'s brief.

## The CLI drives Claude, which is why Svelte is likely promptable

The CLI's agent mode is not a bespoke code generator. Observed in a run's own logs:

```
[INFO] Agent: claude
[INFO] Model: claude-opus-5
[INFO] Starting Claude Agent session
🔧 Using tool: mcp__pencil__execute
```

It runs a **Claude Agent SDK session** against the Pencil MCP tools, defaulting to `claude-opus-5`.
`--agent` also accepts `codex` and `gemini`. So "does pen.dev produce Svelte" is really "can Claude write
Svelte given the canvas" — which reframes #411's first gate from a product limitation into a prompting
question. **The gate still has to be tested**; it is now expected to pass rather than expected to fail.

The CLI session exposed `browser`, `execute`, `get_app_state`, `get_style`, `read_skill` — which at the
time looked like a *different* surface from the desktop server's seven. It was not: the CLI was already
shipping the newer five-tool surface that the desktop app adopted an hour later at v1.2.7. The two agree.
Treat any tool inventory here as a snapshot of a fast-moving product, and re-probe before relying on it.

Flags worth knowing for #411: `--repo, -C <path>` sets the agent's working directory, which is how it
reads a codebase; `--tasks` runs batch operations from JSON; `--in`/`--out` make iteration explicit rather
than in-place; `--usage` writes token cost to JSON.

## Round trip, proven end to end

1. `bunx pen --out smoke.pen --prompt "…a 400x200 white card…" --export smoke.png` — the agent called
   `mcp__pencil__execute` once and wrote both files.
2. The exported PNG matched the prompt: correct size, radius, border, both text runs, colours.
3. `open -a /Applications/Pen.app smoke.pen` — **the desktop app rendered the CLI-created file**, writing
   a fresh preview to `~/.pencil/previews/`.

CLI → `.pen` → desktop render works with no manual step.

## What a `.pen` file actually is

Pretty-printed JSON, one node tree, four-space-free two-space indent, `"version": "2.17"`. The smoke test
produced 47 lines for a card with two text runs. It is genuinely readable and genuinely diffable — the
docs' *"commit .pen files like code files"* holds up.

Two things to weigh when [#412](https://github.com/markgoho/doula-cloud/issues/412) decides whether these
get committed:

- **Node ids are short random strings** (`yQVea`, `pFFsz`, `iDDHX`). Regenerating a design rather than
  editing it will churn every id and produce a diff far larger than the visual change.
- **Each file carries a `fileToken` UUID**, a cloud-side reference. Committing it commits that handle.

Styling is flat hex and numeric literals — `"fill": "#FFFFFF"`, `"cornerRadius": 12`. Nothing in the
smoke file references a variable, so token binding is a deliberate act, not a default. That is exactly
what #411's third gate has to check against `app/src/lib/styles/tokens.css`.

The default output is unmistakably generic: Inter, `#18181B` on `#FFFFFF`, `#E4E4E7` borders, `#71717A`
captions. This is the house AI-default look that [#409](https://github.com/markgoho/doula-cloud/issues/409)
exists to escape, arriving unprompted, and it is the argument for briefing the tool rather than asking it
for a design.
