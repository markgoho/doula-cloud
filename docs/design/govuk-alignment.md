# The GOV.UK Design System, and what Doula Cloud takes from it

The [GOV.UK Design System](https://design-system.service.gov.uk/) is the reference Doula Cloud reaches for whenever a screen has to **ask a person for something, or tell a person something went wrong**. This document says why, draws the line around what is taken, and holds the table that maps their catalogue onto ours.

The decision itself is [ADR-0021](../adr/0021-govuk-is-the-reference-for-service-patterns.md). This document is the living half of it -- the ADR should not change, and the table below changes every time a screen is built.

## Why this system and not another

Most design systems publish components. GOV.UK publishes **decisions**, and it publishes the research that produced them. When their Dates pattern says a memorable date is three text inputs and never a date picker, that is not taste -- it is what happened when they watched people try. A four-month runway to launch cannot re-run that research, and does not have to.

The fit is unusually close, for three reasons:

1. **Doula Cloud is a service, not an app to browse.** A doula opens it to complete a task -- add a Client, send an Invoice, answer a message -- and leave. That is the shape of thing GOV.UK is built for.
2. **It is form-heavy and low-frequency.** A person who adds four Clients a month never builds fluency. Their patterns assume exactly that person.
3. **The users are people mid-crisis, some of the time.** A client filling in a birth plan after a loss, a doula doing admin between two births. GOV.UK's whole posture -- one question at a time, plain words, an obvious way out -- is designed for someone who has no attention to spare.

This is already how the product has been built. [ADR-0018](../adr/0018-templates-are-a-design-system-layer-with-two-named-exits.md)'s Template layer, the intake sequence drawn in `doula-cloud.pen`, and the three-box date of birth in `b7e8ab7` all came from there. This document makes that explicit rather than leaving it as a habit that only holds while someone remembers.

## What is adopted, and what is not

**Adopted: the decision.** What to ask, in what order, on how many pages. How to word a question, a hint, an error. Where the error summary sits and what its heading says. What happens to focus after a submit. Which of these things research says people get wrong.

**Adopted: the behavior.** A back link that really goes back. A `Change` link that returns to the question and comes back. A `Save and come back later` that saves a half-finished record. Three boxes for a memorable date.

**Not adopted: the markup.** No `govuk-*` classes, no Nunjucks, no `govuk-frontend` package. Our components are Svelte, our layout primitives are the custom elements of [ADR-0003](../adr/0003-css-layout-primitives-as-native-custom-elements.md), and the component tiers are Atomic Design per [ADR-0018](../adr/0018-templates-are-a-design-system-layer-with-two-named-exits.md).

**Not adopted: the look.** Transport, the yellow focus bar, the black-and-white chrome, the 1px-on-everything borders. The look is [the design brief](brief.md)'s -- Plum Dusk, evolved -- and the color, type and spacing truth is `app/src/lib/styles/tokens.css`. GOV.UK never overrules a token.

**Not adopted: the content.** Their patterns are written for UK government services. A National Insurance number, a UK postcode lookup, a sort code, a Welsh language toggle and a phase banner have no counterpart here. Those rows are marked N/A below with the reason, so nobody re-derives the same conclusion later.

The one-line version, which is also [the brief](brief.md)'s governing principle: **conventional in pattern and behavior, distinctive in execution.**

## Autocomplete: whose data it is

Every field that asks for a piece of personal information carries an `autocomplete` token, for two reasons. The browser can fill it, so a doula entering her own email or a Client entering her own address costs nothing rather than a re-type. And [WCAG 2.2's 1.3.5, Identify Input Purpose](https://www.w3.org/WAI/WCAG22/Understanding/identify-input-purpose.html), is level AA, so this is a standing accessibility obligation, not a nicety.

**The rule is the token names whose data it is.** A field about the person filling the form in gets the WHATWG token that names it -- `email`, `name`, `tel`. A field where a doula is entering **someone else's** information -- a Client, during intake -- gets `autocomplete="off"`. Offering the signed-in doula's own stored address for a Client's record is a data-entry hazard, and on a shared laptop it is a privacy one. This is [GOV.UK's own answer](https://design-system.service.gov.uk/patterns/addresses/): a form with two subjects keeps them apart rather than letting the browser guess which one it is filling in.

One field is not "whose data", but "which of two things is happening to it": on the two accept-invite screens, the email is either registering a new address (`email`) or naming an existing one (`username`), and the password is either being set (`new-password`) or recalled (`current-password`), depending on which of the two modes the person picked -- both derived from `mode`, not fixed markup. The plain login screens ask for the identifier only, which is always `username` even though the control is `type="email"`: that is the account identifier, not a new self-declaration, and matches how a browser's own credential manager reads a sign-in form.

`autocomplete="off"` is advisory -- browsers ignore it for passwords, and Chrome ignores it in other places too. Stating it is still worth doing, and a browser overriding it is not a defect this project can fix.

`TextInput` and `Textarea` both accept and forward the attribute, so a route never reaches around the atom to set it by hand.

## Patterns

Every pattern in [their catalogue](https://design-system.service.gov.uk/patterns/), and where it stands here. **Aligned** means it is built or drawn and matches. **Open** means it is ours to build and a ticket says so. **N/A** carries its reason.

### Ask users for...

| GOV.UK pattern | Doula Cloud | Status |
| --- | --- | --- |
| [Addresses](https://design-system.service.gov.uk/patterns/addresses/) | `client.Record`'s five address columns; one fieldset, one input per line, no lookup | Drawn -- [#466](https://github.com/markgoho/doula-cloud/issues/466) |
| [Bank details](https://design-system.service.gov.uk/patterns/bank-details/) | Stripe Connect owns this end to end; we never see an account number | N/A -- and it must stay N/A |
| [Dates](https://design-system.service.gov.uk/patterns/dates/) | Date of birth as three text inputs, Month/Day/Year, never a picker | Drawn -- `b7e8ab7` touched [`doula-cloud.pen`](doula-cloud.pen) only, and [#466](https://github.com/markgoho/doula-cloud/issues/466) is open, so nothing in `app/src` asks for a date of birth at all; walked 2026-08-30, which is how the stale **Aligned** was caught |
| [Email addresses](https://design-system.service.gov.uk/patterns/email-addresses/) | Signup, login, invite, Client email | Wording aligned; `autocomplete` set per [the rule above](#autocomplete-whose-data-it-is), built by [#469](https://github.com/markgoho/doula-cloud/issues/469) -- the Client intake fields wait on [#466](https://github.com/markgoho/doula-cloud/issues/466), which must follow the same rule; walked 2026-08-30 |
| [Equality information](https://design-system.service.gov.uk/patterns/equality-information/) | No equality monitoring is collected | N/A |
| [Names](https://design-system.service.gov.uk/patterns/names/) | Given, family and preferred, split | Departed on purpose -- see below |
| [National Insurance numbers](https://design-system.service.gov.uk/patterns/national-insurance-numbers/) | UK-only identifier | N/A |
| [Passwords](https://design-system.service.gov.uk/patterns/passwords/) | Five `type="password"` inputs, no toggle, no stated rule | Open -- [#470](https://github.com/markgoho/doula-cloud/issues/470) |
| [Payment card details](https://design-system.service.gov.uk/patterns/payment-card-details/) | Stripe's own elements render the card fields | N/A -- Stripe's pattern wins inside its iframe |
| [Phone numbers](https://design-system.service.gov.uk/patterns/phone-numbers/) | `client.Record.Phone`, asked on its own page | Drawn -- [#466](https://github.com/markgoho/doula-cloud/issues/466) |

### Help users to...

| GOV.UK pattern | Doula Cloud | Status |
| --- | --- | --- |
| [Check a service is suitable](https://design-system.service.gov.uk/patterns/check-a-service-is-suitable/) | The Hugo marketing site does this job, not the app | N/A here |
| [Check answers](https://design-system.service.gov.uk/patterns/check-answers/) | `templates/CheckAnswers.svelte` -- key / value / Change rows on hairline dividers, `isWide` for a long list | Aligned -- built on [#464](https://github.com/markgoho/doula-cloud/issues/464); its first route is [#466](https://github.com/markgoho/doula-cloud/issues/466); walked 2026-08-30 |
| [Complete multiple tasks](https://design-system.service.gov.uk/patterns/complete-multiple-tasks/) | The Practice landing page's roll-ups are the nearest thing; a task list is not adopted | Considered -- see below |
| [Confirm a phone number](https://design-system.service.gov.uk/patterns/confirm-a-phone-number/) | No SMS is sent, so no code to confirm | N/A |
| [Confirm an email address](https://design-system.service.gov.uk/patterns/confirm-an-email-address/) | Invitation acceptance already proves the address | Aligned by way of the invite flow; walked 2026-08-30 |
| [Contact a department or service team](https://design-system.service.gov.uk/patterns/contact-a-department-or-service-team/) | `/support`, which Stripe also requires | Open -- [#419](https://github.com/markgoho/doula-cloud/issues/419) |
| [Create a username](https://design-system.service.gov.uk/patterns/create-a-username/) | Email is the identifier; there are no usernames | N/A, deliberately |
| [Create accounts](https://design-system.service.gov.uk/patterns/create-accounts/) | Signup, and both accept-invite routes | `autocomplete` built by [#469](https://github.com/markgoho/doula-cloud/issues/469); the password control itself is still Open -- [#470](https://github.com/markgoho/doula-cloud/issues/470); walked 2026-08-30 |
| [Exit a page quickly](https://design-system.service.gov.uk/patterns/exit-a-page-quickly/) | Adopted, **client portal only** -- never the staff side, because a doula is at work and a client may not be | Decided, open to build -- [#472](https://github.com/markgoho/doula-cloud/issues/472) |
| [Navigate a service](https://design-system.service.gov.uk/patterns/navigate-a-service/) | Flat six-item nav, no breadcrumbs, per the brief | Drawn -- [#452](https://github.com/markgoho/doula-cloud/issues/452) |
| [Recover from validation errors](https://design-system.service.gov.uk/patterns/validation/) | Every form that submits: `ErrorSummary` above the `<h1>`, the same string beside the field, `novalidate` so the page refuses rather than the browser. The Engagement detail page is a deliberate exception -- its failures are section-local operation outcomes, not a refused form, so they stay `Notice` where they happen ([#467](https://github.com/markgoho/doula-cloud/issues/467)) | Aligned -- built on [#467](https://github.com/markgoho/doula-cloud/issues/467); walked 2026-08-30 on a really refused `/login` and `/invite` |
| [Start using a service](https://design-system.service.gov.uk/patterns/start-using-a-service/) | The root route still renders SvelteKit's scaffold | Open -- [#357](https://github.com/markgoho/doula-cloud/issues/357) |

### Pages

| GOV.UK pattern | Doula Cloud | Status |
| --- | --- | --- |
| [Confirmation pages](https://design-system.service.gov.uk/patterns/confirmation-pages/) | Outcomes are announced with `Notice` in place, not on their own page | Departed -- see below |
| [Cookies page](https://design-system.service.gov.uk/patterns/cookies-page/) | [ADR-0016](../adr/0016-teaser-analytics-are-cookieless-and-the-channel-rides-on-the-form.md) keeps analytics cookieless; the only cookie is `__session` | N/A |
| [Interruption pages](https://design-system.service.gov.uk/patterns/interruption-pages/) | Nothing yet needs a full-page stop before a consequential step | N/A for now |
| [Page not found](https://design-system.service.gov.uk/patterns/page-not-found-pages/) | `templates/ErrorPage.svelte`'s `notFound` state, rendered by a `+error.svelte` at every chrome boundary | Aligned -- [#471](https://github.com/markgoho/doula-cloud/issues/471); walked 2026-08-30 |
| [Question pages](https://design-system.service.gov.uk/patterns/question-pages/) | The `QuestionPage` Template; one thing per page, legend or label as the `<h1>` | Drawn -- `e994dc7`, [#464](https://github.com/markgoho/doula-cloud/issues/464) |
| [Service unavailable](https://design-system.service.gov.uk/patterns/service-unavailable-pages/) | `templates/ErrorPage.svelte`'s `unavailable` state (503) | Aligned -- [#471](https://github.com/markgoho/doula-cloud/issues/471); walked 2026-08-30 |
| [Step by step navigation](https://design-system.service.gov.uk/patterns/step-by-step-navigation/) | Their pattern is for guidance across many services; our `organisms/StepRail.svelte` is a progress indicator inside one flow, taking step-by-step's anatomy and Complete-multiple-tasks' semantics ([#432](https://github.com/markgoho/doula-cloud/issues/432)'s departure 1) | N/A -- do not confuse the two |
| [There is a problem with the service](https://design-system.service.gov.uk/patterns/problem-with-the-service-pages/) | `templates/ErrorPage.svelte`'s `problem` state (500 and any other unexpected status) | Aligned -- [#471](https://github.com/markgoho/doula-cloud/issues/471); walked 2026-08-30 |

## Components

Their [component list](https://design-system.service.gov.uk/components/) against our Atomic Design set. Where a row says a name of ours, the **behavior** is theirs and the **markup and look** are ours.

| GOV.UK component | Doula Cloud | Status |
| --- | --- | --- |
| [Accordion](https://design-system.service.gov.uk/components/accordion/) | Nothing needs one | N/A for now |
| [Back link](https://design-system.service.gov.uk/components/back-link/) | `molecules/BackLink.svelte`, rendered by `QuestionPage` and `CheckAnswers` above the error summary | Aligned -- built on [#464](https://github.com/markgoho/doula-cloud/issues/464); walked 2026-08-30 |
| [Breadcrumbs](https://design-system.service.gov.uk/components/breadcrumbs/) | A flat six-item nav means there is no hierarchy to breadcrumb | N/A, deliberately |
| [Button](https://design-system.service.gov.uk/components/button/) | `atoms/Button.svelte` | Aligned; walked 2026-08-30 |
| [Character count](https://design-system.service.gov.uk/components/character-count/) | Nothing counts anything | Open -- [#468](https://github.com/markgoho/doula-cloud/issues/468) |
| [Checkboxes](https://design-system.service.gov.uk/components/checkboxes/) | `atoms/Checkbox.svelte` | Aligned; walked 2026-08-30 |
| [Cookie banner](https://design-system.service.gov.uk/components/cookie-banner/) | No cookies to consent to | N/A |
| [Date input](https://design-system.service.gov.uk/components/date-input/) | A **known** date is `TextInput type="date"` wrapping the native control ([#404](https://github.com/markgoho/doula-cloud/issues/404), `65f0974`); a **memorable** date is three inputs, and those are drawn rather than built | Partly open -- the known half is Aligned, the memorable half waits on [#466](https://github.com/markgoho/doula-cloud/issues/466); walked 2026-08-30 |
| [Details](https://design-system.service.gov.uk/components/details/) | No progressive-disclosure component; `<details>` is the whole implementation when one is needed | N/A for now |
| [Error message](https://design-system.service.gov.uk/components/error-message/) | `LabeledField`'s `error` prop, passed by every form; `MembershipFields` renders a group's under its legend. Wording is gated: `formErrors.usage.spec.ts` fails a commit on "please", "valid", "invalid" or "required" in any component or route | Aligned -- built on [#467](https://github.com/markgoho/doula-cloud/issues/467); the message rendered *below* the control it refuses until `198953e` moved it between the hint and the control, found by walking the pages 2026-08-30 |
| [Error summary](https://design-system.service.gov.uk/components/error-summary/) | `molecules/ErrorSummary.svelte`, positioned by `FormPage` and `QuestionPage` as a named Snippet region. Takes focus on appear; an entry's link focuses its control through HTML's own fragment-focusing steps, no script | Aligned -- built on [#467](https://github.com/markgoho/doula-cloud/issues/467); walked 2026-08-30 |
| [Exit this page](https://design-system.service.gov.uk/components/exit-this-page/) | Adopted on the portal. The button is the easy half; what the app leaves behind is the ticket | Open -- [#472](https://github.com/markgoho/doula-cloud/issues/472) |
| [Feedback](https://design-system.service.gov.uk/components/feedback/) | Their footer prompt for feedback on a government service. `/support` is the contact route here | N/A |
| [Fieldset](https://design-system.service.gov.uk/components/fieldset/) | `FormPage` and `QuestionPage` own the fieldset and legend | Aligned; walked 2026-08-30 |
| [File upload](https://design-system.service.gov.uk/components/file-upload/) | Nothing in the app uploads a file; PDFs are generated, not received | N/A for now |
| [Footer](https://design-system.service.gov.uk/components/footer/) / [Header](https://design-system.service.gov.uk/components/header/) | Ours, from [the brief](brief.md) | Departed -- look, not behavior |
| [Generic header](https://design-system.service.gov.uk/components/generic-header/) | Their unbranded header for services that are not GOV.UK. Nearest in spirit to our top bar, and still replaced by it | Departed -- look, not behavior |
| [Inset text](https://design-system.service.gov.uk/components/inset-text/) | `Notice` covers the announcement job | N/A for now |
| [Language navigation](https://design-system.service.gov.uk/components/language-navigation/) | English only | N/A |
| [Notification banner](https://design-system.service.gov.uk/components/notification-banner/) | `atoms/Notice.svelte`, with the same `role="alert"` / `role="status"` split | Aligned; walked 2026-08-30 |
| [Pagination](https://design-system.service.gov.uk/components/pagination/) | Four list endpoints return every row | Open -- [#446](https://github.com/markgoho/doula-cloud/issues/446) |
| [Panel](https://design-system.service.gov.uk/components/panel/) | Belongs with confirmation pages, which are departed from | N/A -- see below |
| [Password input](https://design-system.service.gov.uk/components/password-input/) | Plain `TextInput` with `type="password"` | Open -- [#470](https://github.com/markgoho/doula-cloud/issues/470) |
| [Phase banner](https://design-system.service.gov.uk/components/phase-banner/) | Alpha/beta/live is a government service standard | N/A |
| [Radios](https://design-system.service.gov.uk/components/radios/) | `molecules/RadioGroup.svelte`, which cannot yet carry a hint per option | Partly open -- [#464](https://github.com/markgoho/doula-cloud/issues/464); walked 2026-08-30 |
| [Select](https://design-system.service.gov.uk/components/select/) | `atoms/Select.svelte` | Aligned; walked 2026-08-30 |
| [Service navigation](https://design-system.service.gov.uk/components/service-navigation/) | The staff and portal top bars | Drawn -- [#452](https://github.com/markgoho/doula-cloud/issues/452) |
| [Skip link](https://design-system.service.gov.uk/components/skip-link/) | None anywhere in the app | Open -- added to [#452](https://github.com/markgoho/doula-cloud/issues/452) |
| [Summary list](https://design-system.service.gov.uk/components/summary-list/) | Two things, deliberately: `molecules/DescriptionList.svelte` for a record's facts, and `CheckAnswers`'s own rows where there is a Change action | Aligned -- [#464](https://github.com/markgoho/doula-cloud/issues/464) kept the action row internal to `CheckAnswers` rather than growing a molecule for one consumer; it moves out when a second page wants it; walked 2026-08-30 |
| [Table](https://design-system.service.gov.uk/components/table/) | `organisms/DataTable.svelte` | Partly open -- walked 2026-08-30: aligned at a desktop width, but the six-column Staff table scrolls the whole page sideways at 320px ([#508](https://github.com/markgoho/doula-cloud/issues/508)), and a numeric column cannot be right-aligned ([#509](https://github.com/markgoho/doula-cloud/issues/509)) |
| [Tabs](https://design-system.service.gov.uk/components/tabs/) | Nothing needs one | N/A for now |
| [Tag](https://design-system.service.gov.uk/components/tag/) | `atoms/Badge.svelte` | Aligned; walked 2026-08-30 |
| [Task list](https://design-system.service.gov.uk/components/task-list/) | Not adopted; see the note on multiple tasks below | Considered |
| [Text input](https://design-system.service.gov.uk/components/text-input/) | `atoms/TextInput.svelte` | Aligned; walked 2026-08-30 |
| [Textarea](https://design-system.service.gov.uk/components/textarea/) | Five raw `<textarea>` elements and no atom | Open -- [#468](https://github.com/markgoho/doula-cloud/issues/468) |
| [Warning text](https://design-system.service.gov.uk/components/warning-text/) | `atoms/WarningText.svelte` -- `Notice.svelte:7` stayed `error \| status \| info` and did not grow a fourth variant, since it announces what happened, not what is about to | Aligned -- [#473](https://github.com/markgoho/doula-cloud/issues/473); walked 2026-08-30 |

## Where we depart on purpose

A departure is only legitimate if the reason is written down. These are the five.

**Names are split into given, family and preferred.** GOV.UK asks for a full name in one field, because splitting it breaks for people whose names do not fit the shape. We split because [ADR-0017](../adr/0017-twelve-columns-a-practice-defined-layer-and-an-engagement-that-is-asked-for.md) makes given name the one required column -- a Client can be saved with nothing else -- and because the product addresses a Client by her first name on every screen, which [#463](https://github.com/markgoho/doula-cloud/issues/463) makes a copy rule. A single field cannot do either. **Preferred name** is what carries the risk their guidance is about: a person whose name is not what a document says gets a field for it, and the product uses that field.

**No confirmation pages.** Their pattern gives a completed transaction its own page with a green panel. Doula Cloud announces outcomes in place with `Notice` and leaves the person where they were, because a doula sending an invoice is mid-task and has four more things to do, not finishing a once-a-year interaction with a government. This is a genuine trade -- the outcome is less emphatic -- and it is the right one for a tool someone works in all day. The `Panel` component follows the pattern out.

**No task list, and no "complete multiple tasks".** Their task list is for a long application a person returns to over days. Doula Cloud's equivalent surface is the Practice landing page, which answers *what needs doing today* out of live data -- unsigned contracts, unpaid invoices, unanswered messages -- rather than tracking one person's progress through one submission. A task list would be a second, weaker answer to a question already answered.

**No breadcrumbs.** [The brief](brief.md) settles a flat six-item nav. A flat structure has nothing to breadcrumb, and adding them would imply a hierarchy that is not there.

**A modal `Dialog`, confirming a destructive action in place.** GOV.UK has never published a Dialog/modal component -- their own backlog issue has been open since 2018 and a comment as recent as May 2026 is still asking the team to revisit it. Its load-bearing reason is progressive enhancement: their 2019 acceptance criteria required a modal to still let someone complete their task with JavaScript unavailable, because government services are built to survive that. That requirement does not bind here -- Doula Cloud is `ssr = false` and client-rendered end to end, so every screen already depends on JavaScript to render at all, not only this one; adopting their no-JS floor would mean rebuilding the whole app, not this component. Their separate, real caution is accessibility, and it does not transfer either: a 2023 finding on their own hand-rolled search-page modal showed VoiceOver and TalkBack ignore the `aria-modal` attribute and can swipe past it into the page behind. `molecules/Dialog.svelte` carries none of that risk -- it is the native `<dialog>` opened with `showModal()`, which the browser makes the rest of the page `inert` for at the platform level, not by an ARIA hint a screen reader may or may not honor. Their own documented pattern for confirming a destructive action is in fact a separate yes/no page (HMRC's *add to a list*), and underneath it sits a preference this project knowingly overrides: prefer letting someone undo an action over asking them to confirm it first ("Never Use a Warning When You Mean Undo"), reserving a hard confirm for the genuinely irreversible. Two of the four actions `ConfirmDialog` guards -- ending a Staff member's sessions, revoking a pending Invitation -- are in fact reversible, and an undo-first design was weighed for them. [#473](https://github.com/markgoho/doula-cloud/issues/473) keeps block-over-warn instead, for all four: a hard block with a deliberate, action-named override, the same house rule that already governed every other destructive action in this app before this ticket, so the four stay consistent with each other rather than splitting by reversibility.

## Keeping this table honest

Four rules, so this does not become a document that was true once.

1. **A new screen that asks a person for something checks this table first.** If GOV.UK has a pattern for it, the burden is on departing, not on adopting.
2. **A departure gets a row here with its reason** on the same commit that departs. A departure with no recorded reason is a defect, and it is the only kind of GOV.UK finding this project treats as one.
3. **A row that says Open names a ticket.** No row says "someday".
4. **A row only says Aligned once somebody has seen it render**, at a desktop width and at 320px, and it carries the date that happened. Source is not evidence here. This table's first version was written by reading the code, which is the right way to audit a *decision* -- whether a date is three inputs, whether a page asks one thing -- and the wrong way to audit a *rendering*. Four rows failed the first walk ([#475](https://github.com/markgoho/doula-cloud/issues/475), 2026-08-30) and none of them could have been caught any other way: the error message sat below its control, the six-column table scrolled the page sideways only at 320px, and two rows named a commit or a ticket that no longer said what the row claimed. `LabeledField`'s label did the same thing before them ([#425](https://github.com/markgoho/doula-cloud/issues/425)). A row whose walk date is older than the component it names has not been checked.

What this document deliberately does not do is track GOV.UK's own releases. Their system changes; ours does not have to follow. The table is checked when we build a screen, not when they ship a version.
