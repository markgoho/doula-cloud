# Theme synthesis: the clock

## What this file is for

This is the second stage of ADR-0041 for one theme on the values track. It lays out what each of the nine values books claims about messages sent because time passed rather than because a person acted, whether to a buyer during a trial or to a user inside the product; where the books agree; where they conflict; and what the repo has already decided. It contains no ruling. Page citations follow each read-back's own page rule. The read-backs are in `docs/research/books/`.

## The question

When, if ever, does a message go out because a date arrived rather than because someone did something?

## What each book claims

**The SaaS Playbook** (Walling, 2023). Automated emails on a clock: a referral ask at 60 or 90 days, onboarding sequences over the first sixty days, a founder email within ten minutes of a cancellation (80/20 SaaS Metrics, pp. 151, 159, 161).

**Start Marketing the Day You Start Coding** (Walling, 2023 ed.). Trials that never end, zero emails during a trial, and a fifteen-field signup form leak a funnel; trial emails lifted installs from 25% to 75% in the author's own case ("Small Startup Acquisition", pp. 213, 224, 228). An automated follow-up sequence over weeks or months, sent from a mailing service, converts a subscriber ("#1 Goal", pp. 27–29).

**Make: The Bootstrapper's Handbook** (Levels). Push notifications are valuable if used modestly, and a weekly grouped digest is the model (Launch, p. 80).

**The Best Interface Is No Interface** (Krishna, 2015). Automatic solutions are hard to get right, and the ones that work earned trust over a long, careful track record (ch. 18, pp. 278–280). A system that reaches out proactively on a person's inferred pattern, before being asked, is held up as the better model, illustrated by a thermostat that stops asking and a hospital sensor that alerts hours ahead of a standard monitor (ch. 15, pp. 258, 260–261). The book does not reconcile the two.

**Design for Cognitive Bias** (Thomas, 2020). No position on timed messages as such. Its nearest claim is on framing: the same fact framed two ways has a massive influence that supersedes the fact (ch. 2, pp. 34–35), which bears on what any message says rather than when it is sent.

**Jobs To Be Done** (Ulwick, 2016). No position.

**Landing Page Hot Tips.** No position on timed messages. Its time-limited discount (Tip #24, PDF p. 56) is a page element, carried by the persuasion theme.

**Copyhackers: Uplift.** No position.

**The Almanack of Naval Ravikant** (Jorgenson, 2020). No position.

## Where the books agree

- The three business books that address it all treat a timed sequence to a buyer during a trial as the mechanism that converts, with Walling's 25%-to-75% figure as the only number offered.
- Levels and Krishna agree that unattended automation is dangerous in proportion to how much of it there is: Levels's "modestly", Krishna's "long, careful track record".

## Where the books conflict

- **Who the message is for.** Walling's sequences reach a buyer in a trial; Krishna's examples reach a person the system already serves; Levels's digest reaches an installed user. No book distinguishes the three cases, and their advice differs by case.
- **Proactive versus earned.** Krishna argues for a system that acts before being asked (ch. 15) and, thirteen chapters later, for one that earns the right to act over years (ch. 18).
- **Evidence.** Walling's figure is one product of his own with no control; Levels offers none; Krishna's examples are a thermostat and a hospital monitor, neither a SaaS.

## What the repo already decided

- ADR-0038: every outbox is nudged by an act, and a due date passing is not an act; an Invoice's overdue state is derived at read time, stored nowhere, and notifies nobody; the Client-facing label stays "Not yet paid".
- ADR-0035: a scheduled sweep for an unconnected Stripe account was rejected in favor of a Staff member's own act.
- ADR-0028: the shell has no notification bell; unfinished work appears in a list and never as a badge that pulses.
- ADR-0002: a Web Push notification carries no content; it wakes the service worker, which fetches the message from the BFF.
- ADR-0014: the waitlist lives in Buttondown, outside the stack; it sends one confirmation carrying the pilot-group offer and broadcasts once in January 2027; Kit and Drip were weighed and rejected as funnel automation.
- The **Credit** entry in `CONTEXT.md`: Credits do not expire. `api/internal/billing/dormancy.go` holds a two-year dormancy notice whose comment cites New York's APL 1315(1-b) escheat deadline as the source of the interval.
- #1266, which asked what a Practice with untouched signup Credits hears after two years of silence, was closed as not planned on 2026-09-10; a not-planned ticket decides nothing.
- Open: #361, the confirmation email and how a person says yes to the pilot; #1003, the channel plan through January.
