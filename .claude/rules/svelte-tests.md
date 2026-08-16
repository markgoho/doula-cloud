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
