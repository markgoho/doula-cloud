---
paths:
  - "app/src/**/*.spec.ts"
---

# Svelte Test Conventions (Vitest + vitest-browser-svelte)

Stack-specific rules for `app/src/**/*.spec.ts`. Builds on the general
SIFERS principles in `~/.claude/rules/testing-philosophy.md` (DRY, hide
mechanics, explicit parameters, behavioral focus) — this file's rules win
where they conflict, since that file was written against Angular projects.

## SIFERS: one `setup()` per `describe` block

Prefer a `setup()` function over `beforeEach`/repeated inline `render()`
calls. It centralizes prop construction, gives every test a happy-path
default, and returns only what the test needs to exercise and assert —
`render()` registers its own cleanup, so `setup()` never needs to return a
teardown handle.

```typescript
interface SetupOptions {
  fields?: Field[];
  answers?: Answers;
}

async function setup({ fields = defaultFields, answers = {} }: SetupOptions = {}) {
  const onAnswerChange = vi.fn();
  const onToggleOption = vi.fn();
  await render(PlanInstanceForm, { fields, answers, onAnswerChange, onToggleOption });
  return { onAnswerChange, onToggleOption };
}

it('calls onAnswerChange when the short_text input changes', async () => {
  const { onAnswerChange } = await setup();
  await page.getByLabelText('Support people').fill('Alex');
  expect(onAnswerChange).toHaveBeenCalledWith('names', 'Alex');
});
```

`render()` is async-only as of `vitest-browser-svelte@3` — `setup()` must
be `async` and every call site must `await` it, even when the test doesn't
otherwise await anything before it.

If a spec needs a per-test DOM lookup helper (e.g. reading a value next to
a label), define it alongside `setup()` rather than repeating a DOM-walk
expression (`.element().nextElementSibling`) in every test.

Don't add `setup()` to specs with no real construction to hide — a table
of pure-function assertions (`expect(fn(x)).toBe(y)`) gains nothing from a
setup wrapper.

## Interactions: use `page` locators, never `fireEvent`/`dispatchEvent`

This stack runs `vitest-browser-svelte` against a real headless Chromium
via `@vitest/browser-playwright` (see `app/vite.config.ts`). Interactions
go through `page` locators from `vitest/browser`:

```typescript
await page.getByLabelText('Support people').fill('Alex');
await page.getByLabelText('Consent to photos').click();
await page.getByLabelText('Location').selectOptions('Home');
```

These dispatch genuine, trusted browser events — never use `fireEvent`,
`dispatchEvent(new MouseEvent(...))`, or `new Event(...)` to simulate an
interaction.

**`@testing-library/user-event` does not apply to this stack and must not
be added as a dependency.** `user-event` exists to make jsdom-simulated
interactions behave more like a real browser. This project already tests
against a real browser, so `page` locators are the direct equivalent —
adding `user-event` would mean also adding jsdom + `@testing-library/svelte`
as a second, less-realistic test path alongside this one.

## Assertions: default to accessible queries; `querySelector` is a named exception

Assert what a user actually perceives — sighted, screen reader, or keyboard
— with `page.getByRole`/`getByText`/`getByLabelText` and
`toBeVisible()`/`toBeInTheDocument()`. This is the same bet intrinsic
layout makes on the markup side: correct semantic HTML and modern CSS
(container queries, `:has()`, subgrid) mean the accessible role tree
already carries almost everything worth asserting, with minimal nesting
needed to get there. A test that has to reach past that tree into class
names or DOM structure is often a sign the markup itself nests deeper than
the layout needs — treat it as a prompt to check the component, not only
the test.

`container.querySelector(...)` stays, but only for facts that genuinely
have no accessible signal:

1. **Distinguishing two elements with the identical accessible role and
   name**, where the only difference is which one CSS hides at the current
   viewport — `RecordDetail`'s `.contents-rail` and `.contents-strip` are
   the same links rendered twice, one per breakpoint, and `getByRole('link')`
   cannot tell them apart.
2. **A deliberately non-accessible element** — a placeholder `<div>`
   reserving a grid column's width while loading, with no ARIA role at all
   so it doesn't double up on a `Skeleton`'s own `role="status"` next to it.
   There is nothing for an accessible query to find, because the whole
   point is that nothing is announced there.

Reach for `querySelector` only after confirming there is no accessible
query that says the same thing — never as a shortcut past one that exists.
When it is used, the surrounding comment should say which of the two cases
applies, so the exception reads as deliberate rather than habitual.

## Callback props are the contract, not implementation detail

`testing-philosophy.md` says not to assert on how a mocked collaborator was
called. That rule targets internal collaborators (injected services,
mocked API/network calls) — asserting on *those* couples a test to
implementation.

Svelte 5 callback props (`onAnswerChange`, `onToggleOption`, and similar
`onXxx` props passed into a component) are different: they are the
component's declared public output. Asserting
`expect(onAnswerChange).toHaveBeenCalledWith('names', 'Alex')` after a user
interaction *is* the behavioral, user-facing assertion for that
interaction — it stays.

## A route's content is declared once, in its own `page.fixture.ts`

A route that the continuum sweep measures has a `page.fixture.ts` beside
it (`app/src/routes/routeFixture.ts`, #570). That file is the **only**
place the route's happy-path content is written. A route spec that
declared its own `data`/`detail`/`baseX` object was a second description
of one screen, and the two drift in a known direction: a spec's own
content is polite because it is chosen to make an assertion readable,
and a fixture's is hostile because #537 established that a polite
fixture measures a screen nobody will ever see. The number the sweep
reports and the content the spec asserts on then describe two different
screens (#596).

**The fixture exports its content by name**, beside `fixture` itself:

```typescript
// page.fixture.ts
export const data: PracticeInvoicePage = { … };
export const fixture: RouteFixture = { name: …, component: Page, props: { data }, … };
```

**The spec imports it** and installs the same `page` the check installs,
through the same `toPageState`:

```typescript
import { toPageState } from '../../../routeFixture.js';
import { data, fixture } from './page.fixture.js';

const pageState = vi.hoisted(() => ({
  params: {} as Record<string, string>,
  url: new URL('https://example.test/'),
  data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));
```

`vi.mock` is hoisted above every import, so the object is declared empty
and filled once the imports have run — the route reads `page` inside its
own functions rather than destructuring it at module scope, so the later
write is seen. Installing the fixture's `params` here rather than
retyping them is not cosmetic: a `practiceId` that drifts from the
fixture's makes every `respond(path)` match silently miss.

For a route that fetches, `toApiResponder(fixture)` is the mock
implementation — the fixture is called per request, because a `Response`
body reads once.

**What a spec still owns:**

- **A rendered form of a fixture value.** `'$4,500.00'` for
  `amountCents: 450_000`, `'Mar 1, 2027'` for a due date — the rendering
  *is* the assertion. Naming a value the fixture holds verbatim (a
  Client's name, a note, an id) is the thing to replace.
- **Content that is not the happy path**: an error body, an empty list, a
  second page arriving after an interaction, a variant the screen
  reaches only under a different state.
- **How that content departs from the fixture.** A variant is written as
  a spread — `{ ...data, hasMore: true }` — never as a fresh object that
  re-states the fields it shares.

**A spec never edits a fixture to suit an assertion.** If a spec needs
content the fixture does not hold, that is the fixture's screen changing,
and the sweep measures it too — decide it deliberately rather than
widening the fixture in passing.

A `+layout.svelte` or `+error.svelte` spec sitting in a directory that
happens to hold a `page.fixture.ts` is not covered by this rule: its
subject is not the route the fixture describes.

## A fixture's row set must hold every state a field renders differently

#537's hostile-value rule picks what to put inside one state — the
longest name, the widest amount. It says nothing about which states
exist: a fixture with one row can only ever show one state of every
field at once, and a thin fixture that holds a single Member, a single
Client, a single kind of Request reads as full and is not. #596 found
this by convergence, not by the sweep — the sweep measures whether a
subject needs more room than it is given, and a thin fixture needs
less, not more, so it stays green while measuring a screen no Practice
has (ADR-0025's Fixtures section, [#720](https://github.com/markgoho/doula-cloud/issues/720)).

Two rows, not a row count: one realizing every field's busiest,
longest, most-flagged state together, one realizing every optional or
empty state together. See `staff/page.fixture.ts` and
`clients/page.fixture.ts` for the shape — a Member with no roles beside
one with two, an Invitation that is both expired and undeliverable
beside one that is neither, a Client with every column populated
beside one with every optional column absent. A field whose render
branches a third way (`offers/page.fixture.ts`'s open-vs-decided
Offer, `plan-templates/page.fixture.ts`'s select-vs-plain Field) earns
a third row for that field, not for the fixture as a whole — and only
where the states actually render differently: an Offer's five terminal
states differ only in a Badge's own label and variant, already swept
on the Badge's style-guide page, so none needs its own row here.

No mechanical check enforces this — see ADR-0025's Fixtures section
for why a shape gate was declined rather than built. Assess it by eye
against the field's own render branches, the same way a fixture's
hostile values are chosen by eye against #537.
