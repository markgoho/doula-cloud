# Throwaway UI prototypes

How a throwaway UI prototype is built in `app/` and handed to the founder as a claude.ai artifact. The `/prototype` skill decides what to build; this file says where it goes and how it ships.

**The model is the `prototype/1502-feedback` branch** ([#1502](https://github.com/markgoho/doula-cloud/issues/1502)). It is one commit with no parent, so `git log` shows nothing useful. Read a file with `git show prototype/1502-feedback:<path>`, and see exactly what the branch changed outside its route with:

```sh
git diff origin/trunk prototype/1502-feedback -- app/vite.config.ts app/src/app.html app/eslint.config.js
```

That diff also shows trunk's later changes in reverse; the lines marked `PROTOTYPE (#1502)` are the prototype's.

## Where it lives

- **A branch, `prototype/<issue>-<slug>`, that never merges.** Use the worktree flow without the PR. The worktree pruner keeps an unmerged `prototype/*` branch (`docs/agents/worktree-flow.md`, Cleanup).
- **A top-level route, `app/src/routes/prototype-<slug>/`.** The model holds `+page.svelte`, one component per variant, a state panel, and `fixtures.ts` with the copy and data. It mounts the real organisms (`StaffTopBar`, `PortalTopBar`) with fixture props.
- **Every file says it is a prototype**, with a `PROTOTYPE (#<issue>)` comment.
- **The route's `+layout.ts` has no DEV guard.** The style-guide's guard (`app/src/routes/style-guide/+layout.ts`) returns 404 unless `import.meta.env.DEV`, and `vite build` is a production build, so the guard would empty the artifact. The model's `+layout.ts` is `export {};` with a comment that says why.
- **Variants switch in page state, not in the query string.** The artifact viewer passes no query string, so the model reads `?variant=` only as a start value and switches with a floating bar and the arrow keys. Every placeholder link is a hash link that stays on the route (`#/prototype-<slug>?nav=<name>`).

Run it locally with `bun run dev` in `app/`, then open `/prototype-<slug>`.

## The single-file hash build

Three branch-only changes make `vite build` emit one self-contained HTML file that routes on the hash, so it runs as an artifact:

- `app/vite.config.ts`, inside `sveltekit({ ... })`: `output: { bundleStrategy: 'inline' }`, `router: { type: 'hash' }` and `serviceWorker: { register: false }`; and at the top level, `build: { assetsInlineLimit: 50_000_000 }`.
- `app/src/app.html`, in `<head>`, a script that opens a bare load on the prototype: `if (!location.hash.startsWith('#/')) history.replaceState(null, '', '#/prototype-<slug>');`
- Build with `bunx vite build` in `app/`. The file is `app/build/index.html`.

Publish that file as a claude.ai artifact and post its link on the issue with the commit it was built from. A new version after feedback is a republish to the same artifact.

## The gates a prototype route trips

The pre-commit hook (`scripts/hooks/pre-commit`) runs svelte-check, ESLint and the unit suite on any staged `app/` file. The branch never merges, which is what makes each override below acceptable: none of it reaches trunk.

- **ESLint's production rules.** The model needed four off: `svelte/no-restricted-html-elements` (a raw `<button>` or `<select>` in the switcher bar), `svelte/no-unused-props`, `unicorn/consistent-boolean-name` and `unicorn/prefer-else-if`. The sanctioned override is one block at the end of `app/eslint.config.js`, scoped to `files: ['src/routes/prototype-<slug>/**']`, with a `PROTOTYPE` comment, so every exemption sits in one place.
- **svelte-check.** The model passes it with no override.
- **The route sweep.** `app/src/routes/route-continuum.svelte.spec.ts` discovers every `+page.svelte` outside `style-guide/` and fails one that has no `page.fixture.ts` and no entry in its `UNSWEPT` list. The model added neither, and its commit records only that lint and svelte-check passed, so it does not show how this gate was met. On a prototype branch, add the route to `UNSWEPT` there.
- **The usage specs that read `src/routes/**`.** `command grep -l routes app/src/lib/*.usage.spec.ts` lists them. Each one's failure message says what to change, and names its marker where it has one (`voice:ignore`, `spelling:ignore`). A prototype's copy is often the copy that ships, so fix a failure in the copy before reaching for a marker.
