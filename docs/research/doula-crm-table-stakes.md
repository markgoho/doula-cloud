# Doula / birth-worker CRM table stakes — market survey

Research for GitHub issue #173 ("v1 features: what do competing doula CRMs
treat as table stakes?"). Question: across the tools doula practices actually
run on today, which features appear in essentially all of them (table stakes),
which appear in only some (differentiators), and which appear in none
(potential gaps rather than table stakes)?

**Point-in-time survey, 2026-08-19.** Vendor feature sets and pricing move
quickly; every claim below is dated to the fetch, and the market roster itself
(who is live, who is vapourware) is the part most likely to be stale first.

Altitude: this is a *whether* survey, not a *how* one. It records what exists
in other products. It makes no recommendation about what Doula Cloud should
build or how.

## Bottom line

**Table stakes** — confirmed on first-party pages for at least 7 of the 9
tools counted (SimplePractice is excluded from counting; see Synthesis for the
test and Confidence / gaps for why):

1. A searchable client record with attached notes and files — **9 of 9**.
   Maps to **Client**.
2. Scheduling with a self-serve booking page the client uses, plus staff-side
   availability control — **8 of 9**. Maps to **Visit**.
3. Templated intake forms the client fills in — **8 of 9**. Adjacent to
   **Plan Instance**.
4. Invoicing with online card payment — **8 of 9**. Maps to **Invoice** and
   **Payment**.
5. Document templates plus file storage — **9 of 9**.
6. A client portal — **7 of 9**, with BirthBase a documented absence.
7. A mobile app or explicit mobile-first access — **7 of 9**.
8. Direct staff-to-client messaging inside the product — **6 of 9**, listed
   because the two best-documented tools carry it on every tier and no tool
   documents its absence. Maps to **Message**.

**Near-universal but short of the bar**, and deliberately not promoted:
contracts with binding e-signature (**5 of 9** — universal across the
business-side tools, ambiguous in the clinical ones, which say "forms and
waivers"; maps to **Contract**); automated appointment reminders (**5 of 9**,
SMS usually a paid tier); split/deposit/payment-plan billing (**5 of 9**);
named service packages (**6 of 9**).

**Differentiators** — real but far from universal: insurance claim submission
and superbills; telehealth video; a signed BAA / HIPAA posture; lead capture
and a sales pipeline; automated workflows; AI note generation; mileage
tracking; multi-doula team assignment with backup visibility;
estimated-due-date awareness; a birth plan as a named artifact. Plus one that
cuts across everything: **an internal staff note on a client at all** (maps to
**Care Plan**) is present on every doula-specific and clinical tool and absent
from all three generic ones.

**Notable absences** — nothing on the market appears to offer, so these are
gaps rather than table stakes: a first-class **Engagement** object (an episode
spanning intake through postpartum around one baby); an on-call rota or
backup-doula handoff mechanism; automation driven off an estimated due date;
a **Birth Plan** built for third-party handoff to hospital staff; and an
immutable, permanently retained message record scoped to an episode. The
closest anything comes is BirthFlow's gestational-age-aware calendar and
BirthBase's shared on-call visibility, and both stop well short.

The strategic shape: the generic tools (HoneyBook, Dubsado, Bonsai) are strong
on money and weak on care; the clinical tools (Jane, SimplePractice, Practice
Better) are strong on care and charting but model a patient with visits, not a
pregnancy; the doula-specific tools have the domain vocabulary but are small,
and two of the four show signs of being thinly maintained.

## The roster — what is actually on the market

The ticket named "Doula Tech", "Bumi", "Nurture" and "Kindbody-adjacent
tools". Only one of those resolved:

- **Doula Tech** resolves to **Doula Technology Solutions (D-Tech)**,
  doulatech.ai — real company, but the product is pre-launch (see below), so
  it is surveyed only as a market signal.
- **Bumi** and **Nurture** did not resolve to any live doula practice-
  management product in search. Either they do not exist under those names or
  they are too small to surface. Treated as not on the market.
- No **Kindbody**-adjacent doula practice-management tool surfaced. Kindbody
  is a fertility-benefits care provider, not a vendor of practice-management
  software to independent doulas.

What is actually live and doula-specific: **Doulado**, **BirthFlow**,
**BirthBase**, **PracticeLite**. Discovery also surfaced eDoula.biz, Mobile
Doula, Enginehire and Omnify as adjacent; they were not fetched and are not
claimed on below.

## Doula-specific tools

### Doulado

The most complete doula-specific tool found. Positions itself directly as
"practice management software for doulas".

Feature surface a doula touches, per the vendor's own plan breakdown
([help.doulado.co, "What are the different plans?"](https://help.doulado.co/article/51-what-are-the-different-plans),
primary, fetched Aug 2026) and [doulado.co](https://doulado.co/) (primary,
fetched Aug 2026):

| Surface | Claim | Tier |
|---|---|---|
| Client records | "Unlimited Clients" | Starter |
| Scheduling | "Flexible Scheduling" — "easily schedule appointments with clients, block of your calendar"; Booking Pages | Starter / Premium |
| Messaging | "Client Messaging" | Starter |
| Documents | "Paperwork & Documents" — templates for "contracts, intake forms and preference worksheets, send them to clients, gather input and signatures" | Starter |
| Billing | "Invoicing & Payments" — "Create packages and services, send invoices, track service delivery, and accept payments" | Starter |
| Team | "Team Management" | Premium |
| Compliance | "HIPAA Compliance", with a signed BAA | Premium |
| Telehealth | "Video Appointments" | Premium |
| Insurance | "Claims Submission" to "over 6000 payers nationwide" | Premium |
| SMS | "Client SMS Notifications" | Premium |
| Reporting | "Reporting Options" | Impact |

Pricing: Starter $19/mo, HIPAA Premium $29/mo, Impact custom
([doulado.co](https://doulado.co/), primary). Notably, HIPAA compliance,
telehealth and claims are all gated above the entry tier — a solo doula on
$19/mo has no BAA.

Doula-specific vocabulary is thin: "preference worksheets" is the only
gesture at a **Birth Plan**, and the client record is a generic contact.
Nothing on first-party pages describes an estimated-due-date field, an
on-call rota, or an episode object equivalent to an **Engagement**.

### BirthFlow

The most domain-native tool found, and the only one that treats pregnancy
timing as a modelling concern rather than a note field.

Per [getbirthflow.com/features](https://getbirthflow.com/features/) (primary,
fetched Aug 2026):

- **"A Birth Calendar That Actually Understands Pregnancy"** — tracks
  gestational age "down to the day". This is the closest anything in the
  survey comes to an **Engagement** with an intrinsic timeline.
- **"Collaborative Birth Plans"** — an interactive birth plan builder.
  Nearest analogue to **Birth Plan** as a named artifact, though the page says
  nothing about handoff to hospital staff.
- **"Secure HIPAA Chat"** — client messaging. Maps to **Message**.
- **"Simple, Professional Invoicing"** — invoices and retainer tracking, plus
  doula package setup. Maps to **Invoice**.
- **"Easy Lead & Inquiry Forms"** — embeddable forms, plus a form builder.
- **"Client Education Portal"** — a resource hub, with a **partner portal**
  carrying its own login. The only first-party evidence in this survey of a
  second-person portal login alongside the client.
- **"Automatic Mileage Tracking"** across prenatal visits, hospital shifts and
  postpartum check-ins.
- Postpartum and newborn check-ins with structured notes and milestone
  tracking; **placenta encapsulation tracking**; birth photography portals;
  a contraction timer that logs timing and intensity.
- Explicitly positions itself as replacing Acuity/Koalendar (scheduling),
  Dubsado (CRM) and QuickBooks (invoicing).

Contracts with e-signature and multi-doula team management were **not found**
on the features page. That is landing-page silence, not a stated absence —
BirthFlow publishes no plan-comparison matrix that would make the omission
meaningful, so treat both as unverified rather than absent.

### BirthBase

Explicitly built by doulas for doulas, and the only tool whose client record
names a pregnancy field on the vendor's own page.

Per [birthbase.com/doula-features](https://www.birthbase.com/doula-features)
and [birthbase.com/agency-features](https://www.birthbase.com/agency-features)
(both primary, fetched Aug 2026):

- **Client Database** — "Organizing names, **EDDs**, contact info, medical
  histories, and other vital details". Estimated due date is a first-class
  field, not a note.
- **Team Corner** — "Unique to BirthBase" — invite doulas, group them "by
  location, specialty", "**Assign clients to your doulas**", and "Reference
  other doulas' profiles and **share client data with your backups**". This is
  the strongest first-party evidence of a backup-doula concept anywhere in the
  survey, though it is data sharing, not a coverage rota.
- **Calendar** — "Create and manage visits on the spot"; "Custom-Built for
  Doulas … eliminate multiple calendars"; "Follow schedules in your network to
  see **who's available or on-call**". Again, visibility rather than a rota.
- **Packages** — "the first app to accommodate doulas with different skills
  and services" (photography, yoga, education), with reusable templates and
  client-facing package selection.
- **Documents** — "Safely manage everything from intake forms to contracts to
  visit notes", filled in "directly in the app".
- **Mileage Tracker** with downloadable reports for tax deductions.
- Admin dashboard, checklists, data backup, **offline capability**.

**Caveat on currency.** The agency page lists a **Parent Portal** marked
"*Coming Q1 2022*" — still marked as coming, as fetched in Aug 2026. Read
literally, that is first-party evidence that BirthBase has no client portal
and that the marketing site has not been revised in roughly four years. Treat
BirthBase's roster as possibly stale.

### PracticeLite

A general practice-management product with a doula-specific landing page,
rather than a doula-native tool.

Per [practicelite.com/doulas](https://www.practicelite.com/doulas) (primary,
fetched Aug 2026), the doula-facing claims are:

- "Keep track of your doula clients, their **birth preferences**, and
  **important dates**"; "Store contact information, medical history, and birth
  plans securely".
- "Schedule prenatal visits, birth support, and postpartum care"; "Send
  reminders and manage your availability".
- "**Birth plan templates and storage**"; "Prenatal and postpartum visit
  tracking"; "Create and store birth plans, client notes, and postpartum care
  records".
- "Client communication tools" (named, not detailed).
- "Professional invoicing for doula services" and payment tracking.
- "Secure, HIPAA-compliant client data storage"; "Mobile-friendly".

This is the second tool to name **Birth Plan** as a distinct stored artifact.
No pricing is shown on the doula page, and no plan-comparison matrix was
fetched, so nothing about PracticeLite can be claimed as absent.

### D-Tech / Doula Technology Solutions

Included only as a market signal. Per [doulatech.ai](https://doulatech.ai/)
(primary, fetched Aug 2026), D-Tech is a Detroit organisation combining a
physical doula hub, a home-birth space, training, and a planned "**Doula Tech
App** — HIPAA-compliant platform that simplifies scheduling, billing, and
practice management with AI-driven tools", plus billing, scheduling and
**credentialing** tools.

The site carries "Coming Soon" across multiple sections, "Price TBA" on
training, and states "We're collecting testimonials from our community
members." The software is **not shipping**. The signal worth keeping: the
feature triple a new entrant leads with is scheduling + billing + practice
management, and it adds credentialing — which no shipping tool in this survey
claims.

## Adjacent practitioner tools

These are built for health-and-wellness practitioners generally. A doula using
one is borrowing a clinical tool; the vocabulary is patient/practitioner, and
the episode model is a course of treatment, not a pregnancy.

### Practice Better

The best-documented tool in the survey — its pricing page publishes a full
plan-comparison matrix, which is the only kind of page where an omission
carries evidential weight.

Per [practicebetter.io/features](https://practicebetter.io/features/) and
[practicebetter.io/pricing](https://practicebetter.io/pricing/) (both primary,
fetched Aug 2026). What a doula would touch, and at which tier:

| Surface | Availability across Sprout (free) / Starter / Professional / Plus / Team |
|---|---|
| "Book, reschedule, confirm appointments" | All tiers |
| "Public booking page & widgets" | All tiers |
| "Customizable session reminders" | All tiers |
| Text (SMS) reminders | Plus and Team only (credits on Professional) |
| "Send forms & waivers"; form and note templates | All tiers |
| "Secure messaging (one-on-one)" | All tiers |
| Client Portal | All tiers |
| "Accept & track payments" | All tiers |
| "Create & send invoices" | 1 per client/month on free; unlimited from Starter |
| "Accept deposits & recurring payments" | Professional and up |
| "Create & send superbills" | Professional and up |
| Telehealth video (one-on-one) | Starter and up |
| "Charting — speed up and simplify note-taking with AI-powered charting" | Add-on |
| Google Calendar integration | All tiers |
| HIPAA/PIPEDA/GDPR compliance | All tiers |
| Mobile app | All tiers |
| "Workflow automations & triggers" | All tiers |
| Practitioner licences | 1 on all tiers below Team; 2+ on Team ($155/mo) |
| Client cap | 3 (free) / 10 / 300 / unlimited / unlimited |

Pricing: Sprout free, Starter $35/mo, Professional $69/mo, Plus $99/mo, Team
$155/mo (monthly rates). Also carries ePrescribe, protocols, group sessions,
programs and courses, and a large nutrition/lab integration surface
(Fullscript, Rupa Health, That Clean Life) that is irrelevant to a doula.

The matrix enumerates exhaustively and contains **no** row for a due date,
gestational age, on-call coverage, or an episode-of-care object. Given the
page's completeness, that is a meaningful absence rather than silence.

### Jane App

Per [jane.app/features](https://jane.app/features) and
[jane.app/pricing](https://jane.app/pricing) (both primary, fetched Aug 2026).

Feature surface, in the vendor's own groupings:

- **Scheduling** — "Online Booking" ("Let patients book their own appointments
  online"), staffing schedules, rooms and resources, waitlist with
  notifications, shift and break management, self check-in, "Scheduled Return
  Visit Reminders", "Calendar Subscriptions", and a patient-facing "Jane for
  Clients" mobile app.
- **Care & charting** — "Documentation" ("charts, forms, and surveys"), "AI
  Scribe" ("let AI Scribe take notes for you"), "Intake Forms", "Telehealth",
  "Secure Messaging".
- **Clinic management** — "Jane Payments", "Billing & Insurance" (claims,
  eligibility checks, outstanding balances), "Packages & Memberships", "Jane
  Payroll", "Reporting & Analytics", "Invoicing & Sales".
- **Marketing** — "Jane Websites", "Jane SEO", "Ratings & Reviews".

Plans: Balance $54/mo, Practice $79/mo, Thrive $99/mo. Secure Client Portal,
Secure Messaging, unlimited email reminders and **unlimited SMS reminders** are
on all three tiers; unlimited appointments and unlimited practitioners start at
Practice.

Jane is the most operationally complete tool surveyed, and the only one
bundling payroll. Its model is clinic-shaped: practitioner, patient,
appointment, chart. Nothing pregnancy-specific.

### SimplePractice

**Partially verified.** SimplePractice's own site resisted fetching in this
session — the features index, the client-portal page, the scheduling feature
page and the plan-comparison page each returned either no enumerable content
or 403/404. Only the pricing page yielded first-party detail. Treat this
section as the weakest-sourced in the survey.

Verified first-party from
[simplepractice.com/pricing](https://www.simplepractice.com/pricing/)
(primary, fetched Aug 2026):

- Plans: **Starter $49/mo, Essential $79/mo, Plus $99/mo**.
- Add-ons: **Care Aide** $29.50/mo (bundling "Note Taker, Session Sidekick,
  Treatment Planner (Beta), Client Summaries"), **Note Taker** $17.50/mo,
  **ePrescribe** $24.50/mo plus an $89 setup fee.

Product areas named first-party on
[simplepractice.com/features/faqs](https://www.simplepractice.com/features/faqs/)
(primary, fetched Aug 2026): **Credentialing**, **Client Portal**,
**Telehealth**, **ICD-10 codes**.

One further first-party figure is already recorded in this repo's Stripe
research: SimplePractice charges **3.15% + $0.30** on card payments
(`docs/research/stripe-connect-platform-fee-norms.md`, citing SimplePractice's
own support centre). Scheduling, intake documents, notes and insurance claim
submission are all widely attributed to SimplePractice and are almost certainly
present, but **could not be confirmed on a first-party page in this session** —
they are recorded as unverified in Confidence / gaps rather than asserted.

## Generic client-business tools

These are freelancer/service-business tools. Doulas substitute them because
they solve the money side well. None claims HIPAA compliance or offers a BAA
on the pages surveyed.

### HoneyBook

HoneyBook markets to doulas directly, which makes its doula page a useful read
on what the vendor thinks this buyer wants.

Per [honeybook.com/doula-business-software](https://www.honeybook.com/doula-business-software)
and [honeybook.com/pricing](https://www.honeybook.com/pricing) (both primary,
fetched Aug 2026):

- **"All-in-one clientflow platform"** — "client communication, documents,
  payments, scheduling and more, all in one organized place."
- **Lead tracking** — capture from website, Facebook ads and email, with
  follow-up "with expectant parents".
- **Online invoices** — "set a fixed payment schedule or charge a once-off
  fee". Payment plans are first-class.
- **Proposals** — "Combine your online invoices, contracts and payments into
  one seamless experience."
- **Online contracts** — "Protect your doula business and capture legally
  binding signatures." Maps to **Contract**.
- **Scheduling**, **online payments** (cards and ACH), **automations**,
  **client portal software** ("an organized shared workspace"), **payment
  reminders**, **mobile app** (iOS and Android), free file-setup migration.

Plans: Starter $29/mo, Essentials $49/mo, Premium $109/mo (billed yearly).
Invoices, payments, proposals, contracts, calendar, client portal and basic
reports are on Starter; the **Scheduler**, automations, SMS reminders and team
members (up to 2) start at Essentials; unlimited team members at Premium.
Processing "from 2.7% + 10¢ for cards and 1.5% for ACH".

The whole model is client → project → proposal → contract → invoice. There is
no care record, no notes surface, and no pregnancy vocabulary beyond the
marketing copy.

### Dubsado

Per [dubsado.com/pricing](https://www.dubsado.com/pricing) (primary, fetched
Aug 2026). The plan matrix is exhaustive, so its omissions carry weight.

| Feature | Starter ($335/yr) | Premier ($525/yr) |
|---|---|---|
| Unlimited projects & clients | ✓ | ✓ |
| Invoicing & payment plans | ✓ | ✓ |
| Form & email templates | ✓ | ✓ |
| Client portals | ✓ | ✓ |
| Calendar connection | ✓ | ✓ |
| Email integration | ✓ | ✓ |
| Mobile app access | ✓ | ✓ |
| Active lead capture forms | 1 | Unlimited |
| **Scheduling** | ✗ | ✓ |
| **Automated workflows** | ✗ | ✓ |
| Public proposals | ✗ | ✓ |
| Bookkeeping integration | ✗ | ✓ |
| Zapier integration | ✗ | ✓ |

Additional users cost $25/mo (4–10), $45/mo (11–20), $60/mo (21–30); extra
brand $10/mo. A 21-day free trial runs on Premier.

Two things stand out. First, **Dubsado is the one tool in the survey where
scheduling is not table stakes** — the entry plan has calendar connection but
no scheduler, so a Starter-plan doula still needs Calendly alongside it.
Second, contracts are not a line in this matrix; they are marketed under
proposals, which is a Premier feature. Dubsado's "project" is the nearest
thing any generic tool has to an **Engagement**, but it is a generic container
with no timeline of its own.

### Bonsai

Per [hellobonsai.com/pricing](https://www.hellobonsai.com/pricing) (primary,
fetched Aug 2026). Priced per user: Basic $9, Essentials $19, Premium $29,
Elite $49 per user/month billed annually ($15/$25/$39/$59 monthly; Elite has a
3-user minimum).

What a doula would touch:

- **Basic** — CRM, services, **contracts**, **proposals**, **forms**,
  **scheduling (1 event type)**, unlimited clients and projects, time
  tracking, iOS/Android/macOS apps, automations, data exports.
- **Essentials** — unlimited invoices, **online payments**, **retainer
  invoices**, subscription invoices, **client portal**, expense and income
  tracking, financial overview, bank sync, custom client fields, whitelabel
  branding.
- **Premium** — **client tasks & messaging**, deals pipeline, custom fields,
  profit/productivity reports, Calendly and QuickBooks integrations.
- **Elite** — custom permissions, timesheet locking, expense markup, Xero.

Bonsai is the most billing-centric tool surveyed and the only one where
**messaging is a top-tier feature** rather than a baseline one. Retainer
invoicing is a good structural fit for doula deposits. There is no care
record, no clinical notes, no HIPAA claim, and — like every generic tool — no
pregnancy vocabulary.

## The DIY stack

Spreadsheet + Calendly + Google Docs + Venmo/Stripe. This is the incumbent to
beat for a solo doula, and it is worth being precise about what it does and
does not cover, because it sets the floor for "a practice cannot stop using
their current tool without it."

**Calendly** — per [calendly.com/features](https://calendly.com/features)
(primary, fetched Aug 2026):

- **Connect calendars** — "Sync with **Google, Outlook, and Exchange**
  calendars to avoid double-booking". Conflict-aware sync is the core promise.
- **Set availability** — working hours, buffers between meetings, daily
  meeting caps.
- **Customize event types** — distinct meeting types (e.g. a 30-minute
  one-on-one), which is how a doula would separate a prenatal from a
  postpartum visit.
- **Share your scheduling link** on a website or in email.
- **Automate communications** — "Workflows send email and text reminders to
  reduce no-shows and follow-ups."
- **Qualify before scheduling** — "Create a Routing Form to request
  information from website visitors and present a specific booking page based
  on their responses." This is an intake form of sorts.
- **Payments** — "Add **Stripe or PayPal** to your scheduling flow to collect
  payments".
- Video conferencing links auto-generated for Zoom, Google Meet or Teams.

**Stripe Invoicing** — per [stripe.com/invoicing](https://stripe.com/invoicing)
(primary, fetched Aug 2026): hosted invoice pages delivered by a unique
emailed link or PDF, custom numbering and branding, cards / ACH Direct Debit /
bank transfers / Apple Pay / Google Pay, recurring billing, **automated email
reminders** for due and past-due invoices, Smart Retries dunning, and
automatic ACH reconciliation. The page describes **no client record, no CRM,
and no scheduling**.

**Venmo business profile** — per
[venmo.com/business/profiles](https://venmo.com/business/profiles/) (primary,
fetched Aug 2026): direct Venmo payments, Tap to Pay, and QR-code payments, at
**1.9% + 10¢** direct/QR and **2.9% + 9¢** for Tap to Pay, with no monthly
cost. It is payment acceptance, not a business platform: no client records,
and invoicing is only alluded to as an in-app capability rather than
described. Attractive on price, and the cheapest way into the stack.

**Google Docs / Sheets** carries the client list, the notes, the birth plan
and the contract template. No first-party citation is needed for the claim
that a spreadsheet stores rows.

What the DIY stack genuinely delivers: scheduling with real two-way calendar
sync, reminders, payment collection, invoicing, documents. What it structurally
cannot deliver: a client portal, in-product messaging tied to a record,
e-signature (without adding a sixth tool), any linkage between the calendar
entry and the client's file, and any shared view for a second doula. Every
tool in the survey beats it on integration, not on any single capability.

## Synthesis

**The test used.** "Present in essentially all" means present in essentially
every tool *a doula could plausibly run their whole practice on*. It
deliberately does **not** mean "present in all tools counted equally", because
the clinical tools share a feature set (claims, superbills, charting,
telehealth) that is table stakes in *their* market and absent from the
freelancer tools. Counting naively would promote insurance billing to table
stakes, which would be wrong.

**The denominator is nine tools:** Doulado, BirthFlow, BirthBase,
PracticeLite, Practice Better, Jane, HoneyBook, Dubsado, Bonsai.
**SimplePractice is excluded from the counting** — its feature pages could not
be fetched (see Confidence / gaps), so counting it either way would be
fabrication. Its verified surface is reported separately in its own section.

Every row below states its own count, in the form *n of 9 confirmed
first-party*. A tool is counted only where a first-party page names the
feature; a tool whose pages are silent is counted as unconfirmed, not as
absent. A row clears the table-stakes bar at **7 or more of 9 confirmed with
no documented absence**, or 8+ with one documented exception.

### (a) Present in essentially all — table stakes

| Feature | Domain term | Count | Evidence |
|---|---|---|---|
| Searchable client record with attached notes and files | **Client** | **9 of 9** | Doulado "Unlimited Clients"; BirthFlow CRM; BirthBase "Client Database"; PracticeLite "Keep track of your doula clients"; Practice Better client caps per tier; Jane "Profile Management"; HoneyBook "Unlimited clients and projects"; Dubsado "Unlimited projects & clients"; Bonsai "CRM" (all tiers) |
| Scheduling with a self-serve client booking page, and staff-side availability control | **Visit** | **8 of 9** | Doulado "Booking Pages"; BirthFlow birth calendar; BirthBase "Book Appointments"; PracticeLite "Schedule prenatal visits … manage your availability"; Practice Better "Public booking page & widgets" + "Customize services & availability" (all tiers); Jane "Online Booking" + shift/break management (all tiers); HoneyBook Calendar (Starter) / Scheduler (Essentials+); Bonsai "Scheduling (1 event type)" (all tiers). **Documented exception: Dubsado's Starter plan has calendar connection but no scheduler** |
| Templated intake forms the client completes | adjacent to **Plan Instance** | **8 of 9** | Doulado "intake forms and preference worksheets"; BirthFlow form builder + lead/inquiry forms; BirthBase "intake forms … Editable"; Practice Better "Send forms & waivers" + form templates (all tiers); Jane "Intake Forms"; HoneyBook lead forms; Dubsado "Form & email templates" (all tiers); Bonsai "Forms" (all tiers). Unconfirmed: PracticeLite |
| Invoicing with online card payment | **Invoice**, **Payment** | **8 of 9** | Doulado "Invoicing & Payments"; BirthFlow "Simple, Professional Invoicing"; PracticeLite "Professional invoicing … and payment tracking"; Practice Better "Accept & track payments" + invoices (all tiers); Jane "Jane Payments" and "Invoicing & Sales"; HoneyBook "Online invoices" + card/ACH; Dubsado "Invoicing & payment plans" (all tiers); Bonsai unlimited invoices + online payments (Essentials+). Unconfirmed: BirthBase (packages but no billing claim) |
| Document templates plus file storage | — | **9 of 9** | Doulado "Documents & Templates" + "Files & Resources"; BirthFlow form builder and document surfaces; BirthBase "Documents … Comprehensive and Secure"; PracticeLite "Create and store birth plans, client notes"; Practice Better "Templates" + "Upload & share documents" (all tiers); Jane "Documentation"; HoneyBook "All professional templates" + documents; Dubsado "Form & email templates" (all tiers); Bonsai "All templates" (Essentials+) |
| Client portal | — | **7 of 9**, 1 documented absence | Doulado client portal; BirthFlow "Client Education Portal"; Practice Better Client Portal (all tiers); Jane "Secure Client Portal" (all tiers); HoneyBook "Client portal" (Starter); Dubsado "Client portals" (all tiers); Bonsai client portal (Essentials+). **Documented absence: BirthBase's Parent Portal is still marked "Coming Q1 2022".** Unconfirmed: PracticeLite |
| Direct staff-to-client messaging in-product | **Message** | **6 of 9** | Doulado "Client Messaging" (Starter); BirthFlow "Secure HIPAA Chat"; PracticeLite "Client communication tools"; Practice Better "Secure messaging (one-on-one)" (all tiers); Jane "Secure Messaging" (all tiers); Bonsai "Client tasks & messaging" (**Premium only**). Dubsado substitutes email integration rather than a message thread; HoneyBook claims "client communication … in one organized place" without naming a messaging feature; BirthBase unconfirmed. **Below the bar on a strict count, but listed here because the two most completely documented tools (Practice Better, Jane) both carry it on every tier and no tool documents its absence** |
| Mobile app or explicit mobile-first access | — | **7 of 9** | BirthBase app with offline capability; PracticeLite "Mobile-friendly"; Practice Better mobile app (all tiers); Jane "Patient App"; HoneyBook iOS and Android; Dubsado "Mobile app access" (all tiers); Bonsai iOS/Android/macOS (all tiers). Unconfirmed: Doulado, BirthFlow |

### (a′) Near-universal, but short of the bar

Four features are widely present and would be reasonable to treat as expected,
but do **not** clear the counting test on first-party evidence. They are
separated out rather than quietly promoted:

| Feature | Domain term | Count | Evidence |
|---|---|---|---|
| Contract with binding e-signature | **Contract** | **5 of 9** | HoneyBook "capture legally binding signatures"; Bonsai "Contracts" (all tiers); Dubsado proposals/contracts (Premier); Doulado "gather input and signatures"; BirthBase "intake forms to contracts". The clinical tools ship "forms and waivers" (Practice Better) or "charts, forms, and surveys" (Jane) and do not use the word *contract* on the pages fetched — whether that is the same capability renamed was not resolvable first-party. Universal among the business-side tools; ambiguous among the clinical ones |
| Automated appointment reminders | — | **5 of 9** | Practice Better "Customizable session reminders" (all tiers); Jane "Unlimited Email Reminders" + "Unlimited SMS Reminders" (all tiers); Doulado "Client SMS Notifications" (Premium); HoneyBook "SMS reminders" (Essentials+); PracticeLite "Send reminders". Also Calendly in the DIY stack. **SMS is commonly a paid upgrade.** Unconfirmed: BirthFlow, BirthBase, Dubsado, Bonsai — all four have automation engines, so silence here is weak evidence of absence |
| Split / deposit / payment-plan billing | **Invoice** | **5 of 9** | Dubsado "Invoicing & payment plans"; HoneyBook "set a fixed payment schedule"; Bonsai "Retainer Invoices"; BirthFlow "retainer tracking"; Practice Better "Accept deposits & recurring payments" (Professional+). Structurally important for doula work, where a deposit at booking and a balance before birth is the norm |
| Named service packages at a price | — | **6 of 9** | BirthBase "Packages"; Doulado "Create packages and services"; BirthFlow "doula package setup"; Jane "Packages & Memberships"; Practice Better "Create packages, book multiple sessions" (Professional+); Bonsai "Services" (all tiers). Unconfirmed: PracticeLite, HoneyBook, Dubsado |

### (b) Present in some — differentiators

| Feature | Who has it | Who does not |
|---|---|---|
| Insurance claim submission | Doulado (Premium, "over 6000 payers"), Jane "Billing & Insurance", SimplePractice, Practice Better (via Claim.MD, Professional+) | HoneyBook, Dubsado, Bonsai, BirthFlow, BirthBase, PracticeLite |
| Superbills | Practice Better (Professional+), Jane, SimplePractice | all doula-specific and generic tools |
| Telehealth video | Doulado (Premium), Practice Better (Starter+), Jane, SimplePractice | HoneyBook, Dubsado, Bonsai, BirthFlow, BirthBase |
| HIPAA posture / signed BAA | Doulado (Premium tier only), Practice Better (all tiers), Jane, SimplePractice, PracticeLite, BirthFlow (chat) | HoneyBook, Dubsado, Bonsai — none claims it on the pages fetched |
| **Internal staff notes on a client** (maps to **Care Plan**) | BirthBase "visit notes"; PracticeLite "client notes" and "postpartum care records"; BirthFlow structured postpartum and newborn check-in notes with milestone tracking; Practice Better "Charting" and note templates; Jane "Documentation — charts, forms, and surveys"; SimplePractice progress notes (via its Note Taker add-on). **every doula-specific and every clinical tool** | **HoneyBook, Dubsado, Bonsai — none of the generic tools has a care-record surface at all.** Their client record holds projects, documents and money, not observations. This is the sharpest single split in the survey |
| AI-generated notes on top of charting | Jane "AI Scribe", Practice Better AI charting (add-on), SimplePractice "Note Taker" / "Care Aide" (add-on) | every doula-specific tool; every generic tool |
| Lead capture and sales pipeline | HoneyBook lead tracking, Dubsado lead capture forms, Bonsai "Deals Pipeline" (Essentials+), BirthFlow lead/inquiry forms, Doulado "Lead Forms" | Jane, Practice Better, SimplePractice, BirthBase |
| Automated workflows | Practice Better (all tiers), HoneyBook (Essentials+), Dubsado (Premier only), Bonsai (all tiers) | BirthBase, PracticeLite |
| Multi-doula team with client assignment | BirthBase "Team Corner" / "Assign clients to your doulas", Doulado "Team Management" (Premium), Jane unlimited practitioners (Practice+), Practice Better (Team, $155/mo), HoneyBook (Essentials+), Bonsai (per-user pricing) | Dubsado charges per extra user rather than modelling a team; BirthFlow makes no team claim |
| Backup-doula data sharing | **BirthBase only** — "share client data with your backups" | everyone else |
| Estimated due date as a first-class field | BirthBase ("names, **EDDs**"), BirthFlow (gestational age "down to the day"), PracticeLite ("important dates") | Doulado and all six non-doula tools |
| Birth plan as a named artifact | BirthFlow "Collaborative Birth Plans", PracticeLite "Birth plan templates and storage", Doulado "preference worksheets" | all six non-doula tools |
| Mileage tracking | BirthBase, BirthFlow | everyone else |
| Contraction timer / labour tools | **BirthFlow only** | everyone else |
| Partner / second-person portal login | **BirthFlow only** (BirthBase's is unshipped) | everyone else |
| Payroll | **Jane only** | everyone else |
| Credentialing | SimplePractice; D-Tech promises it (unshipped) | everyone else |
| Offline capability | **BirthBase only** | everyone else |

The differentiator list splits cleanly along one seam: the doula-specific
tools own the *domain* differentiators (EDD, birth plans, mileage, labour
tools, backup sharing) and the established tools own the *operational* ones
(claims, telehealth, payroll, AI charting, automation). No tool holds both
sides.

The **Care Plan** row cuts across that seam and reinforces it. An internal
staff note on a client is present on every doula-specific and every clinical
tool, and absent from all three generic ones. So the generic tools are not
merely missing pregnancy vocabulary — they have no place to record what
happened in a **Visit** at all. A doula who substitutes HoneyBook or Dubsado
is keeping her care notes somewhere else, which puts the DIY stack's Google
Docs back in the picture even for a practice that pays for a CRM.

### (c) Absent across the board — potential gaps, not table stakes

These are stated at the strength the evidence supports. Where the absence
rests on a plan-comparison matrix or full feature index — a page that
enumerates by design — it is called an absence. Where it rests on a marketing
page saying nothing, it is called "not found on first-party pages", which is
the weaker and honest claim.

1. **A first-class episode object spanning intake through postpartum around
   one baby** — i.e. **Engagement**. Every tool models client + appointments.
   The freelancer tools add a generic "project" (Dubsado, Bonsai) which is the
   nearest structural analogue but carries no timeline of its own. BirthFlow's
   gestational-age-aware calendar is the closest anything comes to an episode
   with intrinsic time, and it is a calendar, not a record. *Absence: strong
   for Practice Better, Dubsado and Bonsai (complete matrices, no such row);
   not found on first-party pages for the rest.*

2. **An on-call rota or a backup-doula handoff mechanism.** BirthBase comes
   closest twice — "see who's available or **on-call**" and "share client data
   with your **backups**" — but both are visibility and sharing, not a
   coverage schedule with an owner per window. No tool describes assigning
   on-call duty for a birth window. *Absence: strong for Practice Better,
   Dubsado, Bonsai, Jane (all publish complete scheduling matrices — waitlists,
   shifts, breaks and rooms are enumerated, on-call is not); not found on
   first-party pages elsewhere.* This is the single largest structural gap
   found, and it is the feature a doula practice's actual operating model
   turns on.

3. **Automation driven off an estimated due date.** BirthFlow claims "smart
   recommendations based on pregnancy/postpartum stage", which is the only
   first-party evidence of EDD-derived behaviour anywhere. Practice Better,
   HoneyBook, Dubsado and Bonsai all have automation engines and none is
   described as date-of-birth-relative. *Absence: strong for the tools with
   published automation feature sets.*

4. **A birth plan built for third-party handoff.** Two tools store a birth
   plan (BirthFlow, PracticeLite) and one stores "preference worksheets"
   (Doulado). None describes producing a document for hospital staff who are
   not users of the system — no print-for-handoff, no shareable read-only
   link for a non-account third party. *Not found on first-party pages; no
   tool publishes a matrix granular enough to make this a strong absence.*

5. **An immutable, permanently retained message record scoped to an episode.**
   Secure messaging is table stakes, but no vendor page describes it as an
   append-only record kept as part of a case file. The framing everywhere is
   communication convenience, not record-keeping. *Not found on first-party
   pages.*

6. **Practice-defined form templates as a configurable schema.** Every tool
   has form templates; none of the pages fetched describes a practice
   authoring its own field definitions (typed fields, sections) that then bind
   to every client's plan and snapshot at fill time. Template libraries are
   consistently presented as documents to copy, not schemas to instantiate.
   *Not found on first-party pages — this is a fine-grained distinction that
   marketing pages would not be expected to draw either way, so the evidence
   is weak in both directions.*

Items 1, 2 and 4 are the ones that survive as genuine domain gaps. Items 5 and
6 are distinctions that vendor marketing would not surface even if the
capability existed, so they should not be treated as validated openings
without hands-on trials.

## Confidence / gaps

**High confidence** (full plan-comparison matrix or complete feature index
fetched first-party): Practice Better, Bonsai, Dubsado, HoneyBook, Jane,
BirthBase, Doulado, Calendly.

**Medium confidence** (marketing feature page fetched first-party, but no
enumerating matrix, so absences cannot be asserted): BirthFlow, PracticeLite,
Stripe Invoicing, Venmo.

**Low confidence — explicitly unverified:**

- **SimplePractice.** Its features index, client-portal page, scheduling
  feature page and plan-comparison page all failed to yield first-party
  content (403 / 404 / no enumerable text). Only the pricing page and a
  features FAQ returned anything. **Scheduling, calendar sync, intake
  documents, progress notes, insurance claim submission and secure messaging
  are all unverified for SimplePractice in this survey** — they are widely
  attributed to it and almost certainly present, but no first-party page
  confirmed them here. Any decision that leans on SimplePractice's specific
  feature set should re-verify.
- **BirthFlow contracts, e-signature and team management.** Not on the
  features page; no matrix exists to make the omission meaningful. Unknown.
- **PracticeLite pricing and plan structure.** The doula page shows none and a
  separate `/pricing` page was not fetched.
- **Doulado calendar sync.** Doulado publishes booking pages but no
  first-party statement was found about two-way sync with Google or Outlook
  calendars. Unknown.
- **Two-way calendar sync generally.** Only Calendly ("Sync with Google,
  Outlook, and Exchange … to avoid double-booking") and Practice Better
  (Google Calendar on all tiers) were confirmed first-party. Jane lists
  "Calendar Subscriptions", which is likely one-way iCal rather than two-way
  sync — not resolved. Sync **direction** is therefore not established
  market-wide, and the table-stakes list above claims only "scheduling", not
  "two-way sync".
- **BirthBase currency.** Its site still advertises a Parent Portal as
  "Coming Q1 2022" in Aug 2026. Whether BirthBase is actively maintained was
  not established; its feature roster may not reflect the shipping product in
  either direction.
- **Bumi, Nurture and Kindbody-adjacent doula tools.** Searched and not found.
  Recorded as not on the market, but absence of a search hit is weaker
  evidence than a first-party page would be.
- **Actual doula adoption.** This survey establishes what the tools *do*, not
  what doulas actually *use*. No first-party market-share data was available
  for any vendor. The premise that doulas commonly substitute HoneyBook,
  Dubsado and Bonsai comes from the ticket and from HoneyBook marketing
  directly to doulas — it is not independently verified here.
- **Pricing volatility.** Every price above is a list price fetched Aug 2026,
  several while promotional discounts were running (SimplePractice at 50% for
  three months; Bonsai annual discounting). Do not treat them as durable.

Not surveyed at all, and possibly relevant: eDoula.biz, Mobile Doula,
Enginehire, Omnify, Practice (practice.do), and Agiled — all surfaced during
discovery but not fetched.

## Note on sourcing

Every feature claim above traces to a vendor's own feature page, pricing page,
or public help centre, fetched Aug 2026 and marked `(primary)`. Where a page
could not be fetched or did not enumerate, the claim is marked unverified in
Confidence / gaps rather than asserted or quietly dropped.

Two evidence standards were applied deliberately, because they are not
interchangeable:

- For **presence**, a vendor feature page asserting the feature is sufficient.
- For **absence**, a marketing page's silence is not evidence. Absence is
  asserted only from a page that enumerates exhaustively by design — a pricing
  plan-comparison matrix or a full feature index. Everything else is written
  as "not found on first-party pages".

Third-party sources surfaced during discovery were used **only** to find out
which products exist, and were then discarded in favour of the vendor's own
pages. None is cited as evidence for any feature claim. For the record, the
pages refused as evidence were: agiled.app's "9 Best Client Management
Software for Doulas", bloombirthstudio.com's "5 Best Doula Business Software
Options for 2026", vev.co's postpartum-doula software roundup,
nationalbabyco.com's CRM guide, nicholejoy.com's tech-tools post,
doulabusiness.com's app list, bebomia.com's tools page, and Capterra. Vendor
blog posts arguing for their own product (HoneyBook's "The Best CRM for Your
Doula Business", BirthBase's "Choosing a Doula Software", Doulado's "What is a
CRM") were likewise not used as feature evidence.
