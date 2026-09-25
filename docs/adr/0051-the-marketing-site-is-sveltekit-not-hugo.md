# The marketing site is SvelteKit, not Hugo

The marketing site at `doula.cloud` is a SvelteKit package, `site/`, that prerenders every page to static HTML and ships no JavaScript. It replaced the Hugo site under `hugo/` on [#1467](https://github.com/markgoho/doula-cloud/issues/1467), decided by the founder on 2026-09-25, while the site still held only a placeholder home page, `/pilot-terms`, and the generated Practice pages at `/p/<slug>/`.

**Why.** The site is about to grow ([#284](https://github.com/markgoho/doula-cloud/issues/284), [#358](https://github.com/markgoho/doula-cloud/issues/358), [#419](https://github.com/markgoho/doula-cloud/issues/419), [#868](https://github.com/markgoho/doula-cloud/issues/868)), and every page it gains costs more under Hugo than under the tool the rest of the repo uses:

- **One design system, by import.** The site imports `app/src/lib/styles/app.css` whole: the fonts, the tokens in both themes, and the layout primitives ([ADR-0003](0003-css-layout-primitives-as-native-custom-elements.md)). The Hugo layouts copied six palette values by hand, because Hugo had no asset pipeline to share a stylesheet through.
- **One set of gates.** The spelling sweep, `tokens.usage.spec.ts` and `layout.usage.spec.ts` now read `site/` too, and the site has its own 100% coverage gate. None of them could read a Go template. That removes the reason [ADR-0025](0025-layout-is-verified-across-the-continuum.md) gave for stopping its gate at `app/`.
- **One toolchain.** Bun, Vite, Svelte and Vitest, the same as `app/`. The Hugo binary, and the CI container image that existed only to carry it, are gone.

**What did not change.** Every URL Hugo served is served at the same path: `/pilot-terms/` and `/p/<slug>/`, each a directory with an `index.html`, which is the address Stripe holds for each Practice ([#382](https://github.com/markgoho/doula-cloud/issues/382)) and the one the [#443](https://github.com/markgoho/doula-cloud/issues/443) probe requests. The Practice page keeps `data-practice-page`. `scripts/sync-practice-pages.ts` keeps its `SYNC_PRACTICE_PAGES=required` gate and prunes before it writes; it now writes one JSON file per page into `site/practice-pages/`. The `repository_dispatch` rebuild is unchanged. The site still publishes no RSS feed, no taxonomy pages and no `/p/` index.

**Considered option.** Keep Hugo. It builds thousands of pages in seconds and is one binary with no `node_modules`. Neither matters at this size: the pilot has at most hundreds of Practice pages. Its built-ins (aliases, sitemaps, feeds, markdown content) each have a small SvelteKit equivalent, and a Firebase Hosting `redirects` entry is a real 301 where a Hugo alias is a meta-refresh page.

**Consequences.**

- Whatever a Practice typed reaches the page as JSON data and is printed by a Svelte template, which escapes it. `site/src/routes/p/[slug]/practice-page.svelte.spec.ts` proves this with hostile input.
- A build with no Practice pages is a correct build (local, PR preview). So `site/vite.config.ts` lets `/p/[slug]` go unbuilt, and refuses any other route that does.
- A change to `app/src/lib/styles/` also deploys the site, because the site is built from it.
- The site has about 250 npm packages to keep current, where Hugo had none.
