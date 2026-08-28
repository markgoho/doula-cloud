# App shell and dense-form patterns in comparable practice products

Research for wayfinder ticket
[#406](https://github.com/markgoho/doula-cloud/issues/406), part of map
[#405](https://github.com/markgoho/doula-cloud/issues/405) ("Holistic
application design"). The question: **what do comparable products actually do
for an application shell and for dense, practice-defined forms — and which of
them is worth being compared to?**

Doula Cloud today has essentially no application chrome.
`app/src/routes/+layout.svelte` renders `{@render children()}` with no header,
no nav and no page frame; the only shell anywhere is a `<header>` holding one
right-aligned Sign out button; the staff dashboard at
`practices/[practiceId]` is an `<h1>` and four bare links. The brief that
fixes this cannot be written from nothing, so this document supplies the
references.

**Method.** Public marketing sites, product tours, vendor help centres and
public screenshots only. No signups, no trials, no credentials. Sites that
hide their product shots behind JS were driven with `/playwriter` against the
user's own Chrome. Every screenshot in
[`app-shell-survey/`](app-shell-survey/) was captured from a public page in
August 2026, and every claim below is sourced to the page it came from.

**One caveat on the captures.** The browser used prefers a dark colour scheme,
so a few full-page marketing captures render dark. Every screenshot of an
actual product UI is a vendor-published image and is unaffected.

## Summary

| # | Product | Trade | Shell | Density | Practice-defined fields |
|---|---|---|---|---|---|
| 1 | [Cliniko](https://www.cliniko.com) | Allied health | Fixed dark left sidebar, collapsible, ~11 flat items | Airy content, compact nav | **Yes** — named sections of fields on the record, archive not delete |
| 2 | [Jane App](https://jane.app) | Allied health | Top bar only, 6 tabs, tenant name centred | Compact master-detail | Via chart templates and intake forms, not free record fields |
| 3 | [SimplePractice](https://www.simplepractice.com) | Therapy | Icon-only left rail + top bar, expandable | Airy, generous | **No** — fixed client schema |
| 4 | [Practice Better](https://practicebetter.io) | Nutrition / health coaching | Global rail + a second record-scoped rail | Medium | Indirect — customer builds *forms*, mapped into fixed record sections |
| 5 | [Halaxy](https://www.halaxy.com) | Allied health | — (no public shell shot found) | Compact definition lists | **Yes** — custom questions interleaved with structural facts |
| 6 | [Clio Manage](https://www.clio.com) | Small legal | Top bar + record tab strip | Medium | **Yes** — custom fields grouped into *field sets* bound to matter type |
| 7 | [Doulado](https://doulado.co) | **Doula** | Top bar + section-scoped left rail | **Dense** | Not evidenced publicly |
| 8 | [Enginehire](https://enginehire.io) | **Doula / newborn-care agencies** | — (no product UI published) | — | Not evidenced publicly |
| 9 | [TaxDome](https://taxdome.com) | Small accounting | — (marketing IA only) | — | Custom fields + "organizers" per its own IA |

Products 1–7 carry real observations from published product UI. Products 8–9
are recorded as **surveyed and turned up nothing** — see
[Where the survey found nothing](#where-the-survey-found-nothing).

## The shortlist

Five products are worth being compared to, for five different reasons.

### 1. Cliniko — the closest whole-product analogue

<https://www.cliniko.com>

Cliniko is the reference for the **shell** and for the **practice-defined
field problem at once**, and it is the only product in the survey that solves
the second one the way ADR-0017 already decided to.

![Cliniko sidebar and Settings hub](app-shell-survey/cliniko-shell-and-settings-hub.png)

*Source: [Adding custom patient
fields](https://help.cliniko.com/en/articles/6399827-adding-custom-patient-fields),
help.cliniko.com*

**Shell.** A fixed dark-teal left sidebar, full height, product wordmark at
the top. Eleven flat items, each an icon plus a label: Dashboard,
Appointments, Patients, Invoices, Payments, Products, Expenses, Contacts,
Communications, Reports, Settings, Help. **One level — no nested nav.** The
sidebar collapses via a `←` affordance at its foot. The signed-in user
("Rachel H") sits pinned at the **bottom of the sidebar** with a caret, which
is where account and sign-out live. There is no top bar at all. The tenant is
not named in the chrome — Cliniko is single-practice per account, so it does
not have Doula Cloud's problem of showing which Practice you are in.

**Density.** The nav is compact; the content area is airy — a light grey
canvas with white cards, generous padding, and a comfortable reading measure.
The product is used all day and the density reflects that: nav is tight so
content can breathe.

**Overview page.** The Settings screen is the model for a hub page that is not
a metrics dashboard: **six cards in a 3×2 grid**, each card an illustration, a
heading (Our clinic, Appointments, Patients, Finances, Communication, Data &
docs) and a short list of destination links. It is a link hub that has been
given structure and weight. This is directly transferable to
`practices/[practiceId]`, which today is four bare links.

**Practice-defined fields — the answer.** Cliniko's custom patient fields are
organised as **named sections, each holding an ordered list of typed fields**.
The editor is a stack: a *Section title* row, then *Field title* + *Type*
rows, then per-field *Options* rows where the type needs them, each row
carrying a move-up/move-down control and its own **Archive** button.

![Cliniko custom field section editor](app-shell-survey/cliniko-custom-field-section-archive.png)

Removal is **archive, never delete**. Archived options and fields drop into a
dimmed "Archived options" area with a **Restore** control, and stored values
survive. Cliniko's own documentation states that the new fields appear on the
patient page *"below the Referral source section"* — that is, **appended after
the structural fields on the same page**, not hidden on a separate tab.

![Cliniko custom field settings](app-shell-survey/cliniko-custom-field-editor.jpg)

Two facts here matter to Doula Cloud specifically. First, ADR-0017's
archive-not-delete rule for the Client Field Template is not a novel
invention — it is what the closest analogue already does, for the same reason
(a Client who already holds a value must not silently lose it). Second, the
**section is the unit of pacing**. A wall of practice-defined fields becomes
navigable because the practice is made to name groups before it can add
fields.

### 2. SimplePractice — the reference for form pacing

<https://www.simplepractice.com>

SimplePractice is the reference for **how a long record form is grouped and
paced**, and for the name shape ADR-0017 landed on.

![SimplePractice Edit client form](app-shell-survey/simplepractice-edit-client-form.png)

*Source: [Editing a client's
information](https://support.simplepractice.com/hc/en-us/articles/360049778071-Editing-a-client-s-information),
support.simplepractice.com*

**Shell.** A hybrid: a narrow **icon-only left rail** (calendar, clients,
billing, insurance, reports, notes, settings, then a divider, then
notifications and requests) with a `»` control to expand it to labels, plus a
**top bar** carrying a global "Search clients" field on the left and actions
on the right (analytics, a `+` create button, Messages, an account avatar).
Two levels of chrome, each doing one job: the rail is destination, the top bar
is action and identity.

**Density.** Airy. Generous vertical rhythm, 15–16px labels, roomy inputs, and
a content column that stops well short of the full window width. This is a
product designed to be looked at all day without fatigue, and it spends space
to get there.

**Dense form.** The Edit client screen is the single best pacing reference in
the survey:

- A **breadcrumb** — Clients and contacts / Jamie J Appleseed's profile / Edit
  client — so a deep form still says where it sits.
- A **tab strip inside the record** — Client Info, Billing and Insurance,
  Contacts, Client Portal. The long form is cut into four, and only one is on
  screen.
- Within a tab, **plain bold section headings** (Client Status, Name,
  Clinicians) with a hairline rule between sections. No cards, no boxes — the
  grouping is done with type and space alone.
- **Paired two-column fields** where the fields are short and related (Legal
  first name / Middle name on one row, Legal last name / Suffix on the next),
  dropping to a single column for anything else.
- **Required marked with a red asterisk**, not by marking the optional ones.
- **Hint text under the field it explains**, not in a tooltip.
- **A contextual inline notice that appears on a condition** — ticking "Client
  is a minor" reveals an amber panel telling you to add the responsible person
  as a Contact.

The name shape is worth naming outright, because it is the shape ADR-0017
chose independently: **Legal first name** and **Legal last name** are the only
required fields; Middle name and Suffix are optional; and a separate **"What
name do they go by?"** field carries the hint *"Used in place of first name
across SimplePractice, as well as in client communication (reminders, billing
documents, etc.)."* That is exactly ADR-0017's document-vs-conversation split
between `given_name`/`family_name` and `preferred_name`, down to the
justification.

**Practice-defined fields — an explicit negative.** SimplePractice does not
offer arbitrary custom client fields. The Client Info tab is a fixed schema;
customisation happens in **notes and intake forms**, not on the record itself.
A serious, well-funded product in the nearest adjacent trade decided this
problem was not worth solving on the client record. That is a real data point
against, not an oversight.

### 3. Practice Better — the reference for a record that is too big for one page

<https://practicebetter.io>

Practice Better is the reference for **archetype D (record detail,
multi-section)** and for what a hub page holds when it is not metrics.

![Practice Better client record](app-shell-survey/practicebetter-client-record-sections.png)

*Source: [Working with Client Records in Practice
Better](https://help.practicebetter.io/hc/en-us/articles/41912588997275-Working-with-Client-Records-in-Practice-Better),
help.practicebetter.io*

**Shell.** Two rails. Outside the crop is the global nav; inside it is a
**second, record-scoped left rail** that belongs to this one client. It opens
with an identity block — photo, name, "ACTIVE 2 DAYS AGO", a favourite star, a
row of quick-action icons (edit, photo, message, profile, overflow) — then the
client's key facts as icon-led lines (email, date of birth with computed age,
mobile), then a status chip ("Member"), then the section list: Bookings &
Packages, Billing & Payments, Basic Information, Medical History, Diet &
Lifestyle, Measurements & Vitals, Notes & Recordings.

**The long record is not a long page.** It is a **set of named sections that
are also the nav**. You never scroll past a section you do not care about; you
click past it. This is the strongest available answer to "how is a long record
form paced" once the record is genuinely large.

**Overview page — a feed, not a dashboard.** The default pane when you open a
client is **Recent Activity**: a rich-text composer at the top ("Add a custom
activity or progress note for Mary"), two filter selects (All categories, Any
date), then a reverse-chronological event list — *Current Credit Balance: $50*
with its derivation as bullets, *Form sent to Mary Smith · Secondary Contact ·
EMAIL DELIVERED*, *Session booked for Mary Smith*. Each entry has an icon, a
title, a subtitle, a right-aligned timestamp, and pin/overflow controls.

This is a direct answer to the hub-page question on #405, and it lines up with
this repo's standing cross-cutting expectation that a user can answer *"how
did this thing come to be?"*. Practice Better's hub page **is** the audit
trail, made readable and made the default view.

**Practice-defined fields — indirectly.** Practice Better's customisation is
the **form**, not the record field. The customer builds forms with the
Automatic Form Builder or by hand (question types include multiple choice,
tables, scale and contact information), and **Form Mappings** then take the
answers and write them into named sections of the client record. The custom
layer and the structural record stay separate, and a mapping joins them.

### 4. Clio Manage — the reference for scaling custom fields past a handful

<https://www.clio.com>

Clio matters because it is the only product in the survey that has thought
about what happens when *different kinds of work* need *different custom
fields*.

![Clio product composite](app-shell-survey/clio-product-composite.png)

*Source: [Clio Manage product page](https://www.clio.com/products/manage/)*

**Custom fields — field sets.** Clio's help centre states that custom fields
are *"additional fields of information that you may need to capture for a
contact or matter that Clio does not automatically capture"*, and — the
important part — that *"you can group them together into **field sets** in
order to easily manage several fields that apply to particular situations,
like specific practice areas."* Fields can be marked required and can be made
a default. A matter surfaces them on its **own Custom Fields tab**, visible in
the composite above as one of four tabs on the matter dashboard (Dashboard,
Custom Fields, Activities, Calendar).

![Clio custom fields help article](app-shell-survey/clio-custom-fields-article.png)

*Source: [Create Custom Fields in Clio
Manage](https://help.clio.com/hc/en-us/articles/9285496802331-Create-Custom-Fields-in-Clio-Manage)*

So Clio gives three answers Cliniko does not: fields are **grouped into named
sets**, sets are **bound to a kind of work** rather than applied globally, and
the whole custom layer gets **its own tab** rather than being appended to the
structural form.

That third choice is the one to argue with. ADR-0017 is explicit that a
Client's Practice-defined values are read **live** and are part of her record,
not an annexe to it. A separate tab makes the custom layer feel optional.
Cliniko's append-below-the-core placement matches the ADR better; Clio's
field-set grouping is what to steal.

### 5. Doulado — the only doula-specific product with a public UI

<https://doulado.co>

Doulado is worth being compared to for one reason: it is the closest thing to
a direct competitor whose interface can actually be seen, and it is **much
denser than anything else in the survey**.

![Doulado clients list](app-shell-survey/doulado-clients-list.png)

*Source: [doulado.co](https://doulado.co/) hero image*

**Shell.** A white top bar: wordmark far left, then a global Search field,
then centred section tabs — Home, **Clients** (active, underlined), Templates,
Billing — then three utility icons on the right (one carrying a red badge) and
an avatar with the user's name and a caret ("Bridget Parke ▾"). Below it, a
**section-scoped left rail** that changes with the tab: CLIENTS (with a count
badge), PAPERWORK, CHECKLISTS, NETWORK. Two levels, the second one local.

**The tenant is not shown anywhere in the chrome.** Neither is a practice
switcher. For a 14-doula agency that is a real omission, and it is a
difference Doula Cloud should make deliberately rather than by accident.

**Density — the outlier.** The Clients list is the densest screen in the whole
survey and it is worth studying closely:

- The page has a real header: `h1` "Clients" with a secondary "Lead Forms"
  button and a primary "+ New Client" button right-aligned on the same line.
- A filter row of four controls — a text filter, Assigned To, Label, Any
  Status.
- The table packs **four lines into the Name cell alone**: avatar, name,
  email, date of birth with a computed age, phone.
- Status is a coloured pill *plus* a short record code (`#JDCL2`).
- Labels are **multiple coloured pills per row** — Prenatal Education, Birth,
  Postpartum, Lactation, Pelvic Floor, BCBS, Workshop — doing the work of a
  tag system, in colour, inline.
- The Date column shows a due date **and a computed countdown beneath it**
  ("Due 6/27/2026" / "12 weeks, 6 days"). For birth work the countdown is the
  fact people actually read.
- A Providers column is an **avatar stack**.
- Four icon actions per row.
- A pinned footer bar: "Showing 23 results · Export All" on the left, "View
  Archive · Settings" on the right.

Colour carries meaning throughout — the labels are not decorative. Rows are
tight, roughly 44–48px. Doulado is the evidence that a doula-facing product
can be far denser than the allied-health products and still be legible, and
the countdown column is a domain-specific idea worth taking.

Pricing is public and sizes the market: $19/mo Starter, $29/mo Premium (adds
HIPAA compliance, claims submission, team management), and a custom-priced
"Impact" tier *"for larger agencies or teams that needs reporting and more"*.

## The other four products

### Jane App

<https://jane.app>

![Jane top bar](app-shell-survey/jane-top-bar.png)

![Jane patients master-detail](app-shell-survey/jane-patients-master-detail.png)

*Source: [The Patients Tab: A Snapshot of Your Full Patient
List](https://jane.app/guide/the-patients-tab-a-snapshot-of-your-full-patient-list),
jane.app/guide*

**Shell.** A **top bar only** — no sidebar. Six flat items on the left
(Schedule, Patients, Staff, Billing, Reports, Settings), the product logo and
the **tenant name** ("Demo Clinic") centred, and on the right a **location
switcher** ("The Village ▾"), a `?` help control, and the account menu as an
avatar with the user's name ("Demo Owner ▾"). Jane is the only product in the
survey that puts the tenant in the middle of the chrome, and the only one that
carries a second tenant-scoped switcher (location) beside the account menu.
That is the closest match to Doula Cloud's Practice + Staff-member shape.

**Density.** Compact. The teal bar is roughly 40px tall and the whole shell
costs one band of vertical space.

**Index page — master-detail.** The Patients tab is a narrow left panel
holding a search field, an alphabetical list of names (given name regular,
family name bold, so the sort key reads first), and a **"⊕ New Patient"
button pinned at the foot of the panel**; the rest of the screen is the detail
pane, which shows a grey line-art empty-state illustration until a patient is
picked. Worth noting for archetype C: the create action lives at the bottom of
the list it creates into, not in a page header.

**Practice-defined fields.** Not on the patient record. Jane's customisation
surfaces are the **customisable patient sign-up form** — where a clinic
chooses which of Jane's own fields to include and which to require, per
[Customizable Patient Sign Up
Form](https://jane.app/guide/customizable-patient-sign-up-form) — and chart
templates. Choosing among fixed fields is not the same problem as defining
new ones.

### Halaxy

<https://www.halaxy.com>

![Halaxy patient profile with inline custom fields](app-shell-survey/halaxy-patient-profile-inline-custom.png)

*Source: [Customise your patient
profiles](https://support.halaxy.com/hc/en-au/articles/1500003075021-Customise-your-patient-profiles),
support.halaxy.com*

No public capture of Halaxy's full shell was found — its marketing feature
URLs 404 and the help centre crops to the panel under discussion. What the
help centre does show is the most interesting **placement** decision in the
survey.

**Density — compact definition lists.** The patient profile is read-first: a
header band with avatar, name in caps, a portal status line and an **Edit**
link top-right, then the facts as a tight `label: value` list — Legal Name,
Date of Birth (with computed age), Gender, Review Date, Introduction Source,
**Marital status**, **Children**, Practice Locations, Labels. A status chip
("CURRENT") sits top-right of the body, and Labels render as coloured chips
(Private, New patient, Disability).

**Practice-defined fields — interleaved, not appended.** Marital status and
Children in that list are the *custom* fields. Halaxy renders them **inline in
the same definition list as the structural facts**, in configured order, with
no visual seam and no "Custom fields" heading. A doula reading the record
cannot tell which facts the product knows about and which her Practice added
— which is arguably the right answer for a *live description of a person*, the
exact phrase ADR-0017 uses.

Halaxy also solves the *authoring* side differently: custom questions are
drawn from a **shared question library**, so the questions already used in
clinical tool templates can be reused on the profile, and typing offers
existing questions from a dropdown. Settings expose it as one row in a
definition list — "Customised patient profiles: Default" with a pencil —
alongside Terminology, Introduction Sources, Contact Relationships, Profile
type preferences.

![Halaxy patient profile settings](app-shell-survey/halaxy-patient-profile-settings.png)

### Enginehire

<https://enginehire.io>

![Enginehire marketing page](app-shell-survey/enginehire-marketing-page.png)

*Source: [Doula & Newborn Care
software](https://enginehire.io/newborn-care-specialist-doula-software/)*

Enginehire markets itself as *"ATS + CRM + APP + doula software"*, "the
all-in-one business management doula software, built for newborn care and
doula professionals", and is aimed squarely at **agencies** — the pilot's
shape. Feature blocks name a Client Dashboard, Client Lead Generation, CRM,
Staffing & Scheduling, Payments and invoicing, a Portfolio Builder and a
Candidate Dashboard. Pricing is Free / $40 monthly / $400 yearly.

**No product UI is published.** Every screenshot slot on the page is a lazy
placeholder that never resolves, and there is no tour, no help centre with
screenshots, and no public demo. Recorded here as market context only; nothing
about its shell, density or forms could be observed.

### TaxDome

<https://taxdome.com>

TaxDome was surveyed as a small-accounting comparator. Its `/features/` URL
404s and its product screenshots sit behind a "Take a tour" gate that requires
a signup, which this survey does not do. The only usable public artefact is
its footer information architecture, which is itself mildly interesting for
grouping: Firm management, Client management, Client experience, Revenue
operations. Its client-management list names **CRM, Document management, Tax
organizers, E-signature, Two-way SMS, Activity feed, Client requests** — an
"Activity feed" entry alongside the CRM, echoing Practice Better's choice to
make the feed a first-class destination.

Nothing about TaxDome's shell, density or custom fields could be observed
without a signup.

## Where the survey found nothing

Faithfully recorded, since #405 asked for it:

- **Doula-specific products barely exist as observable software.** Of the
  named doula tools — [Doulado](https://doulado.co),
  [PracticeLite](https://www.practicelite.com/doulas),
  [Enginehire](https://enginehire.io), [eDoula](https://www.edoula.biz),
  [BirthBase](https://www.birthbase.com) — only **Doulado** publishes a real
  screenshot of its own interface. The rest are marketing pages, stock
  photography and pricing tables. The doula trade is served mostly by generic
  tools (Dubsado, HoneyBook, Practice Better) rather than by purpose-built
  software with a design worth studying.
- **Midwifery-specific practice management could not be surveyed.**
  Maternity Neighborhood's practice-management URLs 404 and no public product
  UI was reachable.
- **Home-care agency software could not be surveyed.** Every AxisCare feature
  URL tried returned 404 and no product screenshots were reachable from the
  paths attempted.
- **Nobody in the survey shows a narrow-viewport rendering of their app
  shell.** Not one vendor publishes a mobile or tablet screenshot of the
  *staff-facing* product. Marketing pages show phone mockups of the *client*
  app instead. Whatever these products do when the window narrows, they do not
  consider it a selling point, and it cannot be learned from public sources.
  Doula Cloud will have to decide narrow-viewport behaviour on its own
  reasoning rather than by reference.
- **No vendor documents its type scale, spacing scale or colour tokens
  publicly.** Density had to be read off screenshots, not off a published
  system.

## What this means for the brief

Not decisions — these belong to #409 and the tickets after it — but the survey
points hard in a few directions.

**On the shell.** The field is split. Cliniko and SimplePractice put a rail on
the left; Jane and Doulado put a bar on the top with a second local rail
beneath it. The split correlates with what has to be shown: **the two products
that must display a tenant and a location put it in a top bar**. Doula Cloud
must show a Practice, and a Staff member who can belong to more than one, so
Jane's arrangement — nav left, tenant centre, tenant-scoped switcher and
account right — is the closest fit to the problem. Every product in the survey
puts account and sign-out **behind an avatar menu at one end of the chrome**;
none of them leaves a bare Sign out button on the page, which is what Doula
Cloud does today.

**On nav depth.** Nobody nests. Cliniko is 11 flat items, Jane is 6, Doulado
is 4 plus a local rail. Where a second level exists it is **scoped to the
current section or record** (Doulado's CLIENTS/PAPERWORK/CHECKLISTS, Practice
Better's per-client section list), never a global tree.

**On density.** There is a real spread, and it is not accidental.
SimplePractice and Cliniko are airy and spend space; Doulado is dense and
spends colour. A 14-doula agency reading records all day is closer to
Doulado's problem than to a solo therapist's, but Doulado's density is bought
with heavy coloured pills that would fight any restrained aesthetic. The
honest reading is: **compact rows, airy forms** — Doulado's list density with
SimplePractice's form pacing.

**On the hub page.** Two working answers, and neither is metrics. Cliniko's
Settings grid — cards of grouped destination links — is the answer for a hub
whose job is navigation, and it maps straight onto
`practices/[practiceId]`'s four bare links. Practice Better's Recent Activity
feed is the answer for a hub whose job is *what has been happening*, and it
doubles as the audit trail this repo requires of every feature.

**On practice-defined fields — the hard one.** Four products in the survey
solve it, in four distinguishable ways:

| Product | Where the custom fields live | How they are grouped | Removal |
|---|---|---|---|
| Cliniko | Appended below the structural fields, same page | Practice-named sections | Archive, restorable |
| Halaxy | Interleaved with structural facts, indistinguishable | Configured order, no grouping | Not documented |
| Clio | Their own tab on the record | Field sets bound to a kind of work | Not documented |
| Practice Better | Not on the record — separate forms, mapped in | Whole forms | n/a |

The recommendation the survey supports: **take Cliniko's shape** — named
sections, appended after the structural core on the same page, archive rather
than delete — because it is what ADR-0017 already decided and it is proven in
the nearest trade. **Take Clio's field-set grouping** as the thing to grow
into if one Practice's field list ever gets long enough to need it. **Reject
Clio's separate tab**, which contradicts ADR-0017's "live description of a
person". And note SimplePractice as the explicit dissent: the largest product
in the nearest trade decided not to solve this at all.

## Sources

Every URL below was fetched in August 2026.

- Cliniko — <https://www.cliniko.com>; [Adding custom patient
  fields](https://help.cliniko.com/en/articles/6399827-adding-custom-patient-fields);
  [Custom patient fields are now available in
  Cliniko](https://www.cliniko.com/blog/news/custom-patient-fields/)
- Jane App — <https://jane.app>; [The Patients
  Tab](https://jane.app/guide/the-patients-tab-a-snapshot-of-your-full-patient-list);
  [Customizable Patient Sign Up
  Form](https://jane.app/guide/customizable-patient-sign-up-form)
- SimplePractice — <https://www.simplepractice.com>; [Editing a client's
  information](https://support.simplepractice.com/hc/en-us/articles/360049778071-Editing-a-client-s-information)
- Practice Better — <https://practicebetter.io>; [Working with Client
  Records](https://help.practicebetter.io/hc/en-us/articles/41912588997275-Working-with-Client-Records-in-Practice-Better);
  [Creating Forms & Waivers with the Automatic Form
  Builder](https://help.practicebetter.io/hc/en-us/articles/36961546831643-Creating-Forms-Waivers-with-Practice-Better-s-Automatic-Form-Builder)
- Halaxy — <https://www.halaxy.com>; [Customise your patient
  profiles](https://support.halaxy.com/hc/en-au/articles/1500003075021-Customise-your-patient-profiles);
  [Guide to patient
  profiles](https://support.halaxy.com/hc/en-au/articles/360044499333-Guide-to-patient-profiles)
- Clio — <https://www.clio.com/products/manage/>; [Create Custom Fields in
  Clio
  Manage](https://help.clio.com/hc/en-us/articles/9285496802331-Create-Custom-Fields-in-Clio-Manage);
  [Get Started With Custom
  Fields](https://help.clio.com/hc/en-us/articles/9285493193115-Get-Started-With-Custom-Fields);
  [Matter's
  Dashboard](https://help.clio.com/hc/en-us/articles/16681289917595-Matter-s-Dashboard)
- Doulado — <https://doulado.co/>
- Enginehire — <https://enginehire.io/newborn-care-specialist-doula-software/>
- TaxDome — <https://taxdome.com/>
- Surveyed, no usable public product UI: PracticeLite
  (<https://www.practicelite.com/doulas>), eDoula
  (<https://www.edoula.biz>), BirthBase (<https://www.birthbase.com>),
  Maternity Neighborhood (<https://maternityneighborhood.com/>), AxisCare
  (<https://axiscare.com/>), Karbon (<https://karbonhq.com/product/>)
