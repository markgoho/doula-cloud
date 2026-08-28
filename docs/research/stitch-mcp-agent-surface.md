# Google Stitch: what an agent can drive, and what comes back — fact-finding for wayfinder #407

Research for GitHub issue
[#407](https://github.com/markgoho/doula-cloud/issues/407), a sub-ticket of the
[Holistic application design](https://github.com/markgoho/doula-cloud/issues/405)
map. Stitch is one of the two arms generating aesthetic directions for the
design brief, so this note answers what Stitch can be *made* to do by an agent
before the brief ticket starts.

Investigated 2026-08-28. Two source classes are used and are kept apart:

- **Observed** — I drove it myself, either against the live MCP server at
  `https://stitch.googleapis.com/mcp` with a real API key, or through the Stitch
  web UI in the user's signed-in Chrome. Screenshots and raw JSON were read.
- **Documented** — first-party Google docs at `stitch.withgoogle.com/docs/`
  (served from `app-companion-430619.appspot.com`), the Google Labs blog, the
  MCP Registry, and Google's own `gemini-cli-extensions/stitch` repo.

Anything I could not confirm is marked **NOT VERIFIED** rather than smoothed
over. No claim below rests on a blog post or review article.

## Executive summary

| Question | Answer |
|---|---|
| Is there an MCP server? | Yes. Remote, streamable HTTP, first-party. **Observed working.** |
| Does it work **in Claude Code**? | **No, not today.** It connects, then fails to load a single tool. See §1.4. |
| Can an agent generate a design headlessly? | **Yes. Observed** — 91 s, no browser, over raw JSON-RPC. |
| Can an agent iterate on it? | Calls succeed, **but see the read-back defect below.** |
| Can an agent read the result back? | Partly. First generation, yes. **After an edit, no — observed broken.** |
| Cost to us? | Free. 400 daily credits, no tier, no paywall. **Observed in Settings.** |
| Code export in 7 frameworks? | **Refuted.** One code format: Tailwind-CDN HTML. |
| Figma import? | **NOT VERIFIED** — export exists, no import path found. |
| Multi-screen shared visual system? | Yes, via a first-class Design System object. **Observed.** |

## 1. The MCP server

### It exists and it is first-party

The MCP Registry carries a `com.googleapis` namespace entry published
2026-04-19:

```json
{"name":"com.googleapis.stitch/mcp",
 "description":"Interact with the Stitch API using natural language commands.",
 "version":"1.0.0",
 "remotes":[{"type":"streamable-http","url":"https://stitch.googleapis.com/mcp"}]}
```

Stitch's own docs describe it as a *remote* server, not a local one:

> Most MCP servers you use are Local. They read files on your hard drive or run
> scripts on your machine. Stitch is a **Remote** MCP server. It lives in the
> cloud.
> — [`/docs/mcp/setup`](https://stitch.withgoogle.com/docs/mcp/setup)

**Do not install any npm package.** There is no Google-published
`stitch-mcp` on npm. Every npm package matching that name is third-party.
The server is an endpoint, not a dependency.

### The exact config block for Claude Code

Stitch generates this itself. In the product: avatar → **Stitch Settings** →
**Setup MCP** → Client: **Claude Code**. Claude Code is a first-class entry in
that dropdown, alongside Cursor, Antigravity, VS Code, Gemini CLI, Codex,
OpenCode and Jules. **Observed**; the panel emitted, verbatim except that the
key is redacted here:

```
claude mcp add stitch \
  --transport http \
  --header "X-Goog-Api-Key: AQ.Ab8RN6...REDACTED" \
  https://stitch.googleapis.com/mcp
```

The published docs give the same command with the argument order shuffled:

```
claude mcp add stitch --transport http https://stitch.googleapis.com/mcp --header "X-Goog-Api-Key: api-key" -s user
```

Either form resolves to the same three facts: **URL**
`https://stitch.googleapis.com/mcp`, **transport** `http` (streamable),
**auth** a single `X-Goog-Api-Key` request header. There is no stdio command,
no local binary, no `env` block.

### 1.3 The transport is verified — the server answers

**Observed — handshake, over raw JSON-RPC with exactly the URL and header the
config block specifies:**

```
POST https://stitch.googleapis.com/mcp
{"jsonrpc":"2.0","id":1,"method":"initialize",...}

→ {"result":{"capabilities":{"tools":{"listChanged":false}},
             "protocolVersion":"2025-06-18",
             "serverInfo":{"name":"StatelessServer","version":"scaffolding on HTTPServer2"}}}
```

`StatelessServer` matters: there is no session affinity, so an agent must carry
project and screen IDs itself between calls.

### 1.4 But in Claude Code it connects and then exposes nothing

I ran the config block for real, at `local` scope so no global config or
committed file was touched:

```
claude mcp add --transport http stitch https://stitch.googleapis.com/mcp \
  --header "X-Goog-Api-Key: <KEY>" -s local
claude mcp list
```

**Observed result:**

```
stitch: https://stitch.googleapis.com/mcp (HTTP) - ! Connected · tools fetch failed
        — can't resolve reference #/$defs/ScreenInstance from id #
```

The registration was then removed again (`claude mcp remove stitch -s local`).
Nothing on this machine still carries the key.

So the config block is **correct** — it authenticates and the session opens —
but Claude Code loads **zero** Stitch tools from it. Connected is not usable.

**Root cause, confirmed against the raw `tools/list` payload.** The server ships
a malformed JSON Schema. Seven tools reference `#/$defs/ScreenInstance`, and
`upload_design_md` carries that reference in its `outputSchema` while its
`outputSchema.$defs` is **empty**:

| Tool | `outputSchema` `$defs` present | References `#/$defs/ScreenInstance` |
|---|---|---|
| `create_project` | `ScreenInstance` and 6 others | yes — resolves |
| `get_project` | `ScreenInstance` and 6 others | yes — resolves |
| `list_projects` | `ScreenInstance` and 7 others | yes — resolves |
| `upload_design_md` | **`[]` — none** | yes — **dangling** |

Claude Code validates every schema before registering any tool, so one dangling
`$ref` in one tool takes down the whole server's tool list. My raw JSON-RPC
calls succeeded only because I never validated the schemas.

This is a bug on Google's side, not a configuration mistake, and it is not
something this repo can work around. Until Google populates
`upload_design_md`'s `$defs`, **Stitch is not drivable from Claude Code at
all** — every finding in §3 below was obtained by speaking HTTP to the endpoint
directly. Re-run `claude mcp list` before relying on Stitch; the fix is a
server-side change that needs no action here.

### 1.5 What would need to change to enable it here

Nothing has been left changed. To adopt it later — **once §1.4's schema bug is
fixed upstream** — two edits are needed, and they are separate decisions:

1. `~/.claude.json` → `mcpServers.stitch` gains the HTTP entry above, carrying
   the API key. Consider whether a plaintext key in a global config is
   acceptable, or whether Stitch should be added at `-s project` scope in a
   gitignored `.mcp.json` instead.
2. `~/.claude.json` → `projects["/Users/mgoho/Github/doula-cloud"].enabledMcpjsonServers`
   gains `"stitch"`. Today it is `[]`, so even a registered server stays dark
   in this repo.

Note that the existing `pencil` server is registered globally and is likewise
not enabled here — same pattern, same gate.

## 2. Every tool the server exposes

**Observed** via `tools/list` against the live server: **15 tools**. The
published Reference documents **14** — the live server additionally exposes
`delete_project`. Treat the docs as one behind.

| Tool | Read-only | What it does |
|---|---|---|
| `create_project` | no | Creates a project — the container for screens and code. |
| `get_project` | yes | Project details, including its `designTheme` and a thumbnail. |
| `delete_project` | no | Deletes a project. **Live only, undocumented in the Reference.** |
| `list_projects` | yes | All projects; `filter` accepts `view=owned` (default) or `view=shared`. |
| `list_screens` | yes | Screens in a project. **Returned `{}` in my test — see defects.** |
| `get_screen` | yes | One screen, with `htmlCode`, `screenshot` and `figmaExport` file handles. |
| `generate_screen_from_text` | no | Text prompt → a new screen. Takes minutes. |
| `edit_screens` | no | Text prompt → edits to named existing screens. |
| `generate_variants` | no | 1–5 variants of named screens, steered by `creativeRange` and `aspects`. |
| `create_design_system` | no | Creates foundational tokens — colour, type, roundness, mode. |
| `update_design_system` | no | Updates an existing design system asset. |
| `list_design_systems` | yes | Design systems for a project, or global ones. |
| `apply_design_system` | no | Restyles named screens to a design system's tokens. |
| `upload_design_md` | no | Uploads a base64 `DESIGN.md` — step 1 of 2. |
| `create_design_system_from_design_md` | no | Turns that upload into a design system — step 2 of 2. |

Both generation tools carry the same self-describing warning in their
description, which an agent must obey or it will burn credits:

> This action can take a few minutes to complete. Please be patient. DO NOT
> RETRY.

The docs elaborate: connection errors do not necessarily mean failure, and the
recovery is `get_screen` a few minutes later rather than a re-generate.

### The knobs that matter

- `modelId` — `GEMINI_3_FLASH` or `GEMINI_3_1_PRO`. `GEMINI_3_PRO` is
  deprecated. The web UI's model chip showed **3 Flash** as the default.
- `deviceType` — `MOBILE`, `DESKTOP`, `TABLET`, `AGNOSTIC`.
- `variantOptions` — `variantCount` 1–5 (default 3), `creativeRange` of
  `REFINE` / `EXPLORE` / `REIMAGINE`, and `aspects` drawn from `LAYOUT`,
  `COLOR_SCHEME`, `IMAGES`, `TEXT_FONT`, `TEXT_CONTENT`.
- `DesignTheme` — `colorMode` LIGHT/DARK, three font slots from a 29-family
  enum, `roundness` from four steps, a `customColor` hex seed, a `colorVariant`
  from the Material dynamic-colour set (`MONOCHROME`, `NEUTRAL`, `TONAL_SPOT`,
  `VIBRANT`, `EXPRESSIVE`, `FIDELITY`, `CONTENT`, `RAINBOW`, `FRUIT_SALAD`),
  four optional colour overrides, and a free-text `designMd`.

The font enum is a closed list. **Phosphor is not relevant here — but note that
generated HTML pulls Material Symbols**, which collides with the settled
iconography decision in
[#96](https://github.com/markgoho/doula-cloud/issues/96). Icons must be
replaced on the way in, not adopted.

## 3. Can an agent complete generate → iterate → read back?

**Generate: yes. Iterate: the call succeeds. Read back after an edit: no.**

This is the load-bearing finding of the ticket, so here is the whole run.

### Generate — worked

`create_project` → `projects/1030173294483256667`. Then
`generate_screen_from_text` with a Doula Cloud prompt (staff dashboard, light
theme, warm and trustworthy, DESKTOP, `GEMINI_3_FLASH`). **HTTP 200 in 91.06
seconds.** No browser open, no human involved.

The response is not one artefact. It is a `sessionId` plus six
`outputComponents`:

1. a **`designSystem`** Stitch invented unprompted — displayName
   *"Nurture & Node"*, a long prose `styleGuidelines`, a full `theme`, and a
   complete `DESIGN.md` string;
2. a **`design`** carrying the `Screen` resource;
3. a **`text`** — the agent's prose account of what it did;
4. three **`suggestion`** strings — *"Design the Client List page"*,
   *"Create the Birth Tracking view"*, *"Add a 'New Client Intake' modal"*.

The `Screen` came back complete: `name`, `title` *"Overview Hub"*, 2560×2048,
`deviceType` DESKTOP, `screenMetadata.status` `COMPLETE`, and download URLs for
`screenshot` and `htmlCode`. I downloaded both — a 47 KB PNG and 20 KB of HTML.

### Iterate — the call worked, the result did not persist

`edit_screens` with *"give the unpaid-invoices card the primary container
background, increase its height, and reduce the scale of the other two."*
**HTTP 200 in 16.30 seconds.**

The response is more interesting than expected. It did **not** regenerate the
screen. It returned a `sessionEvent` containing five surgical
`dom_operations`:

```
add_class     section.grid...md\:grid-cols-3 > div:nth-child(3)      bg-primary-container min-h-[240px]
remove_class  section.grid...md\:grid-cols-3 > div:nth-child(3)      bg-[#F4EFE6]
add_class     ...div:nth-child(3) h3, ...                            text-on-primary-container
add_class     ...div:nth-child(1)                                    scale-95 opacity-90
add_class     ...div:nth-child(2)                                    scale-95 opacity-90
```

That is a precise, targeted patch, expressed as CSS-selector-plus-class
mutations. It is *more* granular than the map's "weak granular editing" note
assumes.

### Read back — broken for edits

But the patch never reached anything an agent can read:

- `get_screen` immediately after the edit returned an `htmlCode.downloadUrl`
  **byte-identical to the pre-edit one**.
- I downloaded it. The file is byte-identical, 20 358 bytes both times, and
  `bg-primary-container min-h-[240px]` appears **zero times**.
- I polled `get_screen` three more times at 40-second intervals. The URL never
  changed. This is not eventual consistency inside a two-minute window.
- I then opened the project canvas in the browser. The chat log shows the
  agent's edit summary, but **the rendered screen still shows all three cards
  at equal size** with the unpaid-invoices card in its original neutral fill.

So the observed loop is: **generate → read back works; edit → read back does
not.** The edit exists as a session event and as prose in the chat log; it does
not exist in the artefact any downstream tool consumes.

This matters directly to #405's tool-comparison table, which already records
Stitch as **"reads the correction back: no."** That row is now verified, and
for a sharper reason than expected: the failure is not that a human's canvas
edits are unreadable, it is that *the agent's own edits* are unreadable.

### Two further defects observed

- **`list_screens` returned `{}`** for a project that demonstrably had a
  screen. The only way I obtained a screen ID was by keeping the one
  `generate_screen_from_text` returned.
  **Corrected 2026-08-28 on [#409](https://github.com/markgoho/doula-cloud/issues/409):
  `list_screens` works.** Called against this same project a few hours later it
  returned the full `Overview Hub` screen with its `screenshot` and `htmlCode`
  download URLs. Whatever caused the empty response was transient, not a defect
  in the tool, so an agent *can* discover a project's screens after the fact.
  The `get_project` gap below is unaffected and stays NOT VERIFIED.
- **`get_project` returned no screen instances.** The docs state that
  `apply_design_system` and `create_design_system_from_design_md` take a
  `SelectedScreenInstance.id` *"from `get_project`"*. That field was not in the
  response. So the documented path to applying a design system to existing
  screens could not be exercised. **NOT VERIFIED.**

Between them, these mean an agent cannot reliably *discover* the state of a
Stitch project it did not create in the same session. It must retain IDs from
its own writes.

## 4. Account and quota

**Observed** at `https://stitch.withgoogle.com/settings`, signed in as the
user's own Google account:

> **Usage Today** — Daily Credits **0 / 400**

There is **no tier, no plan selector, no upgrade prompt and no paywall
anywhere in the product.** Nothing gates Stitch behind a Google AI Pro / Google
One subscription, and nothing offers a paid Stitch upgrade. The quota is a flat
daily 400 credits that resets. Google's own Gemini CLI extension README states
the MCP surface is *"free of charge."*

I found **no first-party statement anywhere** tying a Google subscription to a
Stitch limit. The widely-repeated "550 generations/month" and
"350 standard + 200 Pro" figures are secondary-source only and are **NOT
VERIFIED** — the number the product actually shows is 400 daily credits.

Three account facts worth recording:

1. **An API key already existed** on the account before this research —
   created 2026-07-05, last used 29 days ago. Stitch has been used from an API
   before.
2. **The Setup MCP panel renders the key unmasked**, in full, as soon as a
   client is selected. Anyone screen-sharing that page leaks it. Stitch does run
   abuse auto-detection: *"we automatically disable any API keys found to be
   publicly exposed."*
3. **"Allow AI model training" is ON.** The setting reads: *"Let Google use your
   future Stitch conversations to train its generative AI models."*

**Recommendation on (3):** turn it off before Stitch is used for anything
beyond aesthetic exploration. Doula Cloud is a healthcare-adjacent product;
prompts describing a Client intake form, a birth plan or a contract structure
are product design detail we should not be donating as training data by
default. This is a one-checkbox change in Stitch Settings, and it is the user's
call, not mine — I did not touch it. Until it is off, treat every Stitch prompt
as public, and never paste real or realistic client data into one.

## 5. Output — what actually comes back

Four things, and only four.

| Artefact | Form | Where from |
|---|---|---|
| **Screenshot** | PNG at a `lh3.googleusercontent.com` download URL | `Screen.screenshot` |
| **Code** | One HTML file at a `contribution.usercontent.google.com` URL | `Screen.htmlCode` |
| **Figma export** | A `File` handle. **Was `ABSENT` in my `get_screen` response.** | `Screen.figmaExport` |
| **Design system** | A structured `DesignTheme` + a full `DESIGN.md` string | `outputComponents[].designSystem` |

There are **no structured layers**. Nothing resembling a Figma node tree, a
component tree, or an editable object model is exposed. The unit of exchange is
a screen, and a screen is a PNG plus a blob of HTML.

### The "7 export frameworks" claim is refuted

The map records "code export in 7 frameworks (HTML/CSS, Tailwind, Vue, Angular,
Flutter, SwiftUI, React)". **This is wrong.** The in-product Export panel,
**observed** with a screen selected, offers exactly ten options and only one is
a framework:

1. AI Studio *(preview)*
2. Figma
3. MCP
4. Netlify *(preview)*
5. Lovable *(preview)*
6. Bolt *(preview)*
7. `.zip`
8. Code to Clipboard
9. Project Brief
10. Stitch React App

No Vue. No Angular. No Flutter. No SwiftUI. No Tailwind-as-a-target. And no
Svelte, which was never expected. Seven of the ten are handoff destinations, not
formats.

Google's docs say plainly where the framework story actually lives:

> It's important to understand that the HTML code serves as a **base for
> translation**. LLMs excel at taking HTML combined with a reference image and
> converting it to other component formats such as React, Angular, and Vue, or
> non-web platforms such as Jetpack Compose, Flutter, and SwiftUI.
> — [`/docs/learn/overview`](https://stitch.withgoogle.com/docs/learn/overview)

So the "7 frameworks" are an LLM translation step *we* would perform, not
something Stitch emits. In that framing Svelte is no worse off than Vue — both
are one LLM hop from the same HTML.

### What the one HTML export actually is

**Observed**, from the downloaded file. It is Tailwind Play-CDN HTML:

```html
<!DOCTYPE html>
<html class="light" lang="en"><head>
<title>Overview Hub - The Digital Hearth</title>
<script src="https://cdn.tailwindcss.com?plugins=forms,container-queries"></script>
<link href="https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined..." rel="stylesheet"/>
<link href="https://fonts.googleapis.com/css2?family=Playfair+Display...&family=Inter..." rel="stylesheet"/>
<script id="tailwind-config">
  tailwind.config = { darkMode: "class", theme: { extend: { "colors": {
    "on-primary-container": "#551905", "outline-variant": "#dac1ba", ... } } } }
</script>
```

Characteristics that decide the Svelte question:

- **Utility classes in markup, essentially no stylesheet.** One `<style>`
  block, one inline `style` attribute, everything else expressed as Tailwind
  classes like `hidden md:flex flex-col h-screen w-64 border-r border-outline-variant bg-surface-container p-4 space-y-2`.
- **Semantics are thin but present** — one each of `header`, `nav`, `main`,
  `aside`, three `section`, one `h1`, one `h2`, four `h3`, eight `button`.
  Structure is usable; it is not a div soup, but it is not carefully
  levelled either.
- **The colour tokens are Material 3 names** — `surface`, `surface-container`,
  `on-surface`, `on-surface-variant`, `primary`, `primary-container`,
  `on-primary-container`, `outline`, `outline-variant`, and the whole `-fixed`
  family. Fifty-odd of them.
- **Fonts and icons come from Google CDNs** — Playfair Display and Inter from
  Google Fonts, icons as Material Symbols.

### Which export is least bad for Svelte 5 with scoped styles

**Answer: none of the code exports. Take the design system, not the HTML.**

The choice is not among seven frameworks; it is between two artefacts Stitch
produces, and the HTML is the weaker one.

- **Tailwind-CDN HTML is actively hostile to scoped component styles.** Its
  entire styling lives in `class=` attributes. Svelte's `<style>` block scopes
  *CSS rules* to a component; there are no CSS rules here to scope. Porting it
  means reading each utility string and re-expressing it as CSS — which is
  translation, not conversion, and the HTML gives no leverage over just reading
  the PNG.
- **"Stitch React App" is worse.** React idioms would have to be unwound before
  Svelte idioms could be applied. It adds a hop.
- **The `DESIGN.md` / `DesignTheme` is the real deliverable.** It arrives as
  YAML front matter of exactly the shape a token file wants:

```yaml
---
name: Nurture & Node
colors:
  surface: '#fff8f6'
  surface-container: '#f9ebe6'
  on-surface: '#211a18'
  primary: '#944931'
  primary-container: '#d67d61'
  on-primary-container: '#551905'
  outline-variant: '#dac1ba'
  ...
typography:
  display:      { fontFamily: Playfair Display, fontSize: 48px, fontWeight: '600', lineHeight: '1.2', letterSpacing: -0.02em }
  headline-lg:  { fontFamily: Playfair Display, fontSize: 32px, fontWeight: '600', lineHeight: '1.3' }
  body-md:      { fontFamily: Inter, fontSize: 16px, fontWeight: '400', lineHeight: '1.6' }
  label-md:     { fontFamily: Inter, fontSize: 14px, fontWeight: '600', lineHeight: '1.2' }
```

Name-for-name, that is a CSS custom-property block. `surface: '#fff8f6'` becomes
`--surface: #fff8f6`. The typography scale is already a set of named steps with
family, size, weight, line-height and tracking. Converting it is mechanical and
lossless, and it lands in exactly the layer Doula Cloud styles from.

The `styleGuidelines` prose that ships alongside it is the other half — it is
the design brief in draft. From my one run, unprompted:

> The design system is centered on the concept of "The Digital Hearth." It
> balances the empathetic, traditional role of a doula with the modern
> efficiency required for practice management. […] It avoids the clinical
> coldness often found in healthcare SaaS […] The UI should feel like a
> well-organized journal—reliable, private, and gentle.

with concrete rules under it: a 12-column grid capped at 1280px, an 8px baseline
grid, desktop/tablet/mobile margins of 40/32/20px, and depth via "tonal
stepping" rather than shadows.

**So the recommended intake path is: PNG for the look, `DESIGN.md` for the
tokens and the prose, and the HTML only as a structural hint that a human or an
agent reads and discards.** Nothing Stitch emits should be committed.

## 6. How granular is the editing, concretely

The map assumed "weak" and accepted it. The truth is sharper and stranger: the
*instruction* granularity is fine, the *canvas* granularity is nil, and the
*persistence* is broken.

**The canvas has no sub-screen selection at all.** Every documented hotkey
operates on whole screens. From
[`/docs/learn/controls`](https://stitch.withgoogle.com/docs/learn/controls), the
complete edit vocabulary is: Undo, Redo, Copy, Paste, Duplicate *("clone
selected screens")*, Delete, Select All. The tools are Select (`V`), Pan (`H`)
and Zoom (`Z`), and Select's job is *"Click to select screens / Drag to move
screens"*. There is no "click this button, change its padding". The atomic unit
you can grab with a mouse is an entire screen. That is the concrete shape of
"weak": it is not a limited property panel, it is **the absence of any property
panel**, because there is nothing smaller than a screen to select.

**Sub-screen change is therefore only expressible in prose.** The documented
loop is: select the screen, click Edit, Add to Chat, write a targeted sentence.
Google's own house style for that sentence is
*target + visual instruction + UI/UX keyword*:

> Update the **pricing table** to emphasize the middle card. Increase its
> container height and add a drop shadow. Reduce the scale of the sibling cards
> to create a clear **visual hierarchy**.

with the standing advice to *"make one major change at a time."*

**And the model honours it precisely.** My `edit_screens` call used exactly that
formula and got back five correct, minimal `dom_operations` targeting
`div:nth-child(3)` and its siblings. Stitch understood "emphasise the third
card, de-emphasise the other two" and expressed it as a diff rather than a
rewrite. That is genuinely good.

**But it did not stick.** See §3. The precise edit vanished between the
response and every readable artefact.

The one structured, non-prose editing surface that *does* exist is **Edit
Theme** — mode, accent colour, corner radius, font. That is the `DesignTheme`
object, and it is reachable from the MCP server via `update_design_system` and
`apply_design_system`. It is coarse by design: it changes the whole system, not
one element.

Practical read: **treat a Stitch screen as immutable once generated.** To get a
different design, generate again or generate variants. Do not build a workflow
that depends on accumulating edits, because the accumulation was not observable.

## 7. Multi-screen and one shared visual system

**Yes, and this is Stitch's strongest capability for our purposes.**

A **Design System is a first-class object**, not an implicit style. It has its
own resource name (`assets/{asset}`), its own version counter, and four tools
(`create_`, `update_`, `list_`, `apply_design_system`). `apply_design_system`
takes a *list* of screens and restyles all of them to one system's tokens.

**Observed:** Stitch created one unprompted on my very first generation.
"Nurture & Node" was invented from the prompt, attached to the project's
`designTheme`, and laid out on the canvas as its own board next to the
dashboard — showing Primary / Secondary / Inverted / Outlined button states and
type specimens. So the canvas holds screens *and* the system that governs them,
side by side.

The three follow-up suggestions Stitch offered were all next screens
(*"Design the Client List page"*, *"Create the Birth Tracking view"*, *"Add a
'New Client Intake' modal"*), which is the documented multi-screen behaviour
from the March blog post: *"Stitch can automatically generate logical next
screens based on the click, mapping out user journeys effortlessly."*

**`DESIGN.md` is the bridge in the other direction.** Google positions it
explicitly as the design counterpart to `AGENTS.md`:

| File | Who reads it | What it defines |
|---|---|---|
| `README.md` | Humans | What the project is |
| `AGENTS.md` | Coding agents | How to build the project |
| `DESIGN.md` | Design agents | How the project should look and feel |

`upload_design_md` + `create_design_system_from_design_md` let an agent push a
`DESIGN.md` *into* Stitch and have every subsequent screen obey it. There is a
documented spec, a CLI and linting rules under `/docs/design-md/`. I did **not**
exercise the upload path — `get_project` never gave me the
`SelectedScreenInstance.id` the second step requires, so **NOT VERIFIED**.

That path is the interesting one for #405 regardless: it would let Doula Cloud's
own tokens drive Stitch's generation, rather than only harvesting Stitch's
inventions. Worth a follow-up ticket.

## 8. Claims from the map, checked

| Claim on #405 / #407 | Verdict |
|---|---|
| MCP server added | **Confirmed** — and working over raw HTTP. **But broken in Claude Code** (§1.4). |
| AI-native infinite canvas | **Confirmed** — observed; pan/zoom/select tools, screens laid out spatially. |
| Multi-screen generation | **Confirmed** — documented and observed via next-screen suggestions. |
| Figma **export** | **Confirmed** — an Export panel option, and a `figmaExport` field on `Screen`. It was `ABSENT` in my response, so the field is conditional. |
| Figma **import** | **NOT VERIFIED.** No import path found in the product, the docs, or the tool list. |
| Code export in **7 frameworks** | **Refuted.** One code format (Tailwind-CDN HTML) plus a React app export. See §5. |
| Update dated **2026-03-19** | The first-party post is dated **2026-03-18**. Minor, but the 19th is a secondary-source date. |
| Free, no separate tier | **Confirmed** — 400 daily credits, observed, no paywall. |
| Granular editing weak | **Confirmed, with nuance** — see §6. Prose targeting is good; canvas granularity is zero; persistence is broken. |
| `list_screens` broken | **Withdrawn** — see §3. It works; the empty response was transient. |
| "Reads the correction back: no" | **Confirmed, and worse than assumed** — even the agent's own edits are not readable back. |

## 9. What this means for the tool decision

Stitch is a strong **generator** and a poor **surface**.

- As an aesthetic-direction arm for the design brief, it is very good and it is
  free. One 91-second call produced a named direction, a full token set, a
  typography scale, a prose rationale, and a rendered screen. That is exactly
  the job #405 assigns it.
- **But not from Claude Code, today.** The schema bug in §1.4 means an agent in
  this harness cannot call a single Stitch tool. Until Google fixes it, Stitch
  is a **web-UI tool driven by a human**, or a tool reached by a script speaking
  HTTP to `stitch.googleapis.com/mcp` directly. Neither is the "agent generates,
  human edits, agent reads back" workflow #405 is shopping for — and §3 shows
  the read-back half would fail even if the tools loaded.
- As a working surface it fails the round trip, and now for a documented,
  reproducible reason rather than an assumption. Do not plan any workflow that
  reads an edit back out of Stitch.
- **Take the `DESIGN.md`, not the HTML.** The single most valuable artefact is
  the YAML token block plus the `styleGuidelines` prose. Both convert cleanly
  into the repo's own layers. The Tailwind HTML converts into nothing useful and
  should not be committed.
- **Two things must be stripped on the way in**: Material Symbols icons, which
  contradict the settled Phosphor decision in #96, and Google-CDN font links.

### Loose ends worth their own tickets

1. **Turn off "Allow AI model training"** in Stitch Settings before Stitch sees
   any real product detail. One checkbox; the user's decision.
2. **Verify the `DESIGN.md` upload path** (`upload_design_md` →
   `create_design_system_from_design_md`) once the `SelectedScreenInstance.id`
   can be obtained. It is the only route by which Doula Cloud's tokens could
   drive Stitch rather than the reverse.
3. **Decide the MCP registration scope** if Stitch is adopted — global
   `~/.claude.json` with a plaintext key, versus a gitignored project
   `.mcp.json`. Moot until (4) lands.
4. **Re-check the Claude Code tool-load bug** (§1.4) before any ticket plans on
   agent-driven Stitch. One `claude mcp list` answers it. Consider reporting
   `upload_design_md`'s empty `outputSchema.$defs` to Google — it is a
   one-field fix that currently blocks every MCP client that validates schemas.

## Artefacts from this run

- Stitch project: `projects/1030173294483256667` —
  [Doula Cloud research #407](https://stitch.withgoogle.com/projects/1030173294483256667).
  Left in place deliberately; the "Nurture & Node" design system it produced is
  usable input for the design-brief ticket.
- Screen: `projects/1030173294483256667/screens/e3df97c1e311439995ccefcad58c885d`
  — "Overview Hub", DESKTOP, 2560×2048.
- Credits consumed: one generation and one edit, against a 400/day allowance.

## Sources

**Observed in Claude Code** — `claude mcp add --transport http` at `local`
scope, then `claude mcp list`, then `claude mcp remove`. Result quoted verbatim
in §1.4.

**Observed in-product / against the live API** — Stitch Settings
(`https://stitch.withgoogle.com/settings`), the Setup MCP panel, the project
canvas and its Export panel, and `https://stitch.googleapis.com/mcp` via
`initialize`, `tools/list`, `create_project`, `generate_screen_from_text`,
`edit_screens`, `get_screen`, `get_project`, `list_screens`.

**First-party documentation** —
[`/docs/mcp/setup`](https://stitch.withgoogle.com/docs/mcp/setup) ·
[`/docs/mcp/reference`](https://stitch.withgoogle.com/docs/mcp/reference) ·
[`/docs/mcp/guide`](https://stitch.withgoogle.com/docs/mcp/guide) ·
[`/docs/learn/overview`](https://stitch.withgoogle.com/docs/learn/overview) ·
[`/docs/learn/controls`](https://stitch.withgoogle.com/docs/learn/controls) ·
[`/docs/learn/design-modes`](https://stitch.withgoogle.com/docs/learn/design-modes) ·
[`/docs/design-md/overview`](https://stitch.withgoogle.com/docs/design-md/overview) ·
[`llms.txt`](https://stitch.withgoogle.com/llms.txt)

**Other first-party** —
[MCP Registry entry](https://registry.modelcontextprotocol.io/v0/servers?search=stitch) ·
[gemini-cli-extensions/stitch](https://github.com/gemini-cli-extensions/stitch) ·
[google-labs-code/stitch-sdk](https://github.com/google-labs-code/stitch-sdk) ·
[google-labs-code/stitch-skills](https://github.com/google-labs-code/stitch-skills) ·
[blog.google, 2026-03-18](https://blog.google/innovation-and-ai/models-and-research/google-labs/stitch-ai-ui-design/) ·
[blog.google, 2026-05-19](https://blog.google/innovation-and-ai/models-and-research/google-labs/stitch-updates/)

Note that the docs at `stitch.withgoogle.com/docs/` are a single-page app inside
a cross-origin `gapi` iframe. `curl` and WebFetch return an empty shell. The
readable origin is `https://app-companion-430619.appspot.com/docs/<path>/index.html`,
which serves static Starlight HTML.
