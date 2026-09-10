# The Best Interface Is No Interface: the argument, the three principles, and where the book stops

Source: Golden Krishna, *The Best Interface Is No Interface: The Simple Path to Brilliant Technology* (New Riders, 2015; ISBN 978-0-133-89033-4). Read in full on 2026-09-10 for [#1268](https://github.com/markgoho/doula-cloud/issues/1268).

Chapter numbers come from the book's table of contents. Page numbers come from the book's own index, so a claim carries a page only where the index covers it; otherwise it carries the chapter alone. Quotations are short phrases, because the book is under copyright.

## What this file is for

This is the evidence behind the principles tagged **NI** in `docs/manifesto.md`. The manifesto holds the rules a person or an agent checks while building. This file holds what the book actually argued, including its exceptions and its weak spots, so a principle can be checked against its source.

Nothing here is a Doula Cloud decision. Where the repo has already decided something the book touches, the ADR or ticket that decided it is named, and the decision belongs to that record, not to the book.

## 1. The claim

Krishna uses "interface" to mean a graphical user interface: the buttons, menus and form fields on a screen, not a doorknob (chapter 1). His thesis is the book's title, and chapter 8 closes on four lines that state it as a design goal: the best design "reduces work", the best computer is "unseen", the best interaction is "natural", and the best interface is no interface (p. 80).

The claim is about the best *outcome*, not the only one. Chapter 20 says so directly: the phrase means the best possible outcome is no interface, and insisting it is the only viable one would be "utterly stupid" (p. 208). He offers it as a design philosophy that works as a filter for ideas, in the way "less is more" works for Modernism, not as an absolute rule.

## 2. The problem (chapters 2 to 8)

**Screen-based thinking (chapter 2).** The industry answers every problem with an app. The worked example is a car maker's phone app for unlocking a car door. From walking up to the car to opening the door, Krishna counts thirteen steps, eleven of them spent on the phone. A proximity key that unlocks when the driver pulls the handle takes two steps, and he says anything beyond those two "should be frowned upon" (pp. 8-13). This is the book's basic method: list every act between the person and the goal, then ask which acts exist only because of a screen.

**Slap an interface on it (chapter 3).** Cars with web browsers in the dashboard, refrigerators that update social media, a touchscreen recycling bin and touchscreen vending machines are all offered as innovation (pp. 35-44). The screen was added to the object; the object's problem was not solved.

**UX is not UI (chapter 4).** Job listings merge "UX/UI" into one role (pp. 45-46). Krishna lists what UI is (navigation, drop-downs, error messages, notifications) against what UX is (solving problems, understanding needs, efficiency), and argues that hiring someone to make UI produces "more UI, not better UX" (pp. 46-47).

**Addiction UX (chapter 5).** Products paid for by advertising are measured on time spent and monthly active users, so their designers are paid to lengthen a task rather than finish it. His counter-position: the designer's job is to give people what they need "as quickly and as elegantly" as possible and "take you away from technology" (chapter 5).

**Distraction (chapter 6).** Clifford Nass's Stanford research found that heavy media multitaskers were worse, not better, at filtering distraction and switching tasks (pp. 59-61). Interfaces built to hold attention make people less able to focus.

**Screen insomnia (chapter 7).** Screens emit light near daylight color temperature, which suppresses melatonin at night. The chapter links this to sleep and, more tentatively, to cancer risk. Krishna himself calls the cancer connection "not conclusive"; see section 7.

**The screenless office (chapter 8).** The dream of a paperless office took decades to arrive, and paper use rose before it fell (pp. 73-79). He quotes Paul Saffo that change goes "slower than everyone expects" (p. 77) and Don Norman (1990) that interfaces "get in the way" of the job a person is actually trying to do. Screens have replaced paper as the thing that fills working life, and the chapter proposes dreaming of a screenless world the way people once dreamed of a paperless one.

## 3. Principle one: embrace typical processes instead of screens (chapters 9 and 10)

**Back pocket apps (chapter 9).** Phones beg to be taken out: notifications arrive by the hundred, people check their phones well over a hundred times a day by the figures he cites, and most people have felt a phone vibrate when it did not (pp. 86-96). The alternative is software that does its work while the phone stays in a pocket:

- A step-tracking app, Moves, worked in the background with no extra hardware. Krishna praises the start and faults the finish, because it only dumped what it collected into a dashboard to dig through (p. 101).
- A smart deadbolt, Lockitron, first needed twelve steps through a phone app. Its second version unlocked by Bluetooth as its owner reached the door, phone untouched (pp. 102-104).
- Self-checkout machines moved a cashier's work onto shoppers, who got lost in the screens. Krishna reads the higher theft rate as confusion more than intent (pp. 104-106).
- Square's Auto Tab opened a tab as a regular customer walked in, so a cafe could greet them by name with their usual order and no one touched a phone (pp. 106-108). It embraced the customer's typical process but not the business's. The book says Square struggled to teach enough cafes the system, and at its biggest partner the feature never shipped.

**Lazy rectangles (chapter 10).** Design research usually starts well, with observation, interviews and analytics. Then the creativity stops: "insight + insight + insight = screen" (p. 113). Krishna quotes Alan Cooper that the most effective tool is a precise description of the user "and what he wishes to accomplish" (p. 114), and notes that what people wish to accomplish is almost never "to use a menu".

Two paired examples make the point:

- A car app won a CES innovation award for its clean five-button wireframe, including a trunk button (pp. 114-115). Ford's design team instead watched people approach a trunk with their arms full. They put a sensor under the bumper that opens the trunk on a foot kick, and nearly half of Escape buyers paid for the option (pp. 115-116).
- An electric car's app let owners pre-cool the car fifteen minutes before returning, which assumes they remember a hot parked car in the middle of a movie (pp. 117-120). Mazda's 1991 929 used a sun-powered fan triggered by a heat sensor to ventilate the parked car with no interaction (pp. 120-121). Krishna is candid that it only brought a 167°F interior down to about 140°F; Toyota's 2009 Prius version did better (pp. 121-122).

The method he draws from both chapters: observe the person where the task happens, find the typical process, count the steps from the person to the goal, and design so the process works without a screen.

## 4. Principle two: leverage computers instead of serving them (chapters 11 to 13)

**Computer tantrums (chapter 11).** Machines that outcompute chess grandmasters still behave like a two-year-old: password rules, mandatory fields, rigid date formats, expired sessions and error messages softened with "Oops!" (pp. 131-133). The relationship is inverted: "We. Serve. Them." He asks for software that serves people instead.

**Machine input (chapter 12).** Most software gathers information the way paper forms did. Krishna proposes **machine input**, his own term, as the antithesis of user input: the system gathers the signal itself through sensors, location, radios or other software (pp. 139-140). He roots it in Mark Weiser's 1991 essay, "the most profound technologies are those that disappear" (pp. 137-138), and in Xerox PARC's Active Badge, which opened doors and routed calls from a worn badge with no screen at all (pp. 139-140). Later examples: a caving headlamp that senses the light bouncing back and sets its own brightness, so a glance from a cave wall to a glossy map does not blind its wearer (pp. 141-142), and a skullcap that signals a hard hit to the head as it happens, instead of an app about concussions consulted afterward (pp. 144-145). His summary instruction is to let the computer do the search and filter the results, and to stop using drop-downs wherever possible.

**Analog and digital chores (chapter 13).** People are busy, and some problems are not information problems. Everyone knows tires should be inflated; they are not, because doing it is an unpleasant chore. The answer is not a tire maker's brochure app but a tire that inflates itself while driving (pp. 148-150). Washing machines offer so many cycles that people leave one pressed forever. Krishna ties that to Barry Schwartz's paradox of choice, and prefers a dishwasher "sensor cycle" that measures the load and picks the settings (pp. 151-154).

Software has created a second class of chore, "the maintenance of our digital lives": updates, password resets, notifications to clear, folders to sort, flights to check in to (p. 155). Doing them breeds more of them, as a reply to every email brings more email (pp. 155-156). The counterexamples remove a chore instead of managing it: TripIt read forwarded travel emails, and later found them itself, then built the itinerary into the calendar (pp. 156-157). The chapter's rule: because people are forgetful, fragile and busy, computers should do the things people do not want to do, do not know they should do, and are not able to do.

## 5. Principle three: adapt to individuals (chapters 14 and 15)

**Computing for one (chapter 14).** Software is designed for an average that describes nobody, and its value decays from the first day of use until a redesign forces everyone to relearn it (p. 164). Krishna's alternative is software that learns from each person's own patterns, so its value grows with use. His example is LinkedIn's People You May Know, which an employee had to fight to ship in an ad-sized box and which then drove much of the site's growth (pp. 169-170). He also complains that most of the talent able to build such systems is spent on ad targeting (pp. 167-168).

**Proactive computing (chapter 15).** Talking to a computer looks like the future. In practice voice input is a command prompt spoken aloud: a guessing game over a small set of words that work (pp. 175-176). Krishna proposes moving from "Hello, computer" to "Thank you, computer": from answering commands to acting ahead of need. The Nest thermostat began as a screen on a wall. It became a learning system that sets the temperature from sensed habits, with no further input (pp. 176-177).

The chapter's central example applies all three principles in order. Hospital monitors can raise up to 150 alerts a day, and alarm fatigue has been linked to patient deaths (p. 178). EarlySense puts a sensor under the mattress, fitting how a patient already lies in bed (principle one). It reads heart rate, breathing and movement with no electrodes (principle two). It compares each patient against their own pattern and alerts the right clinician for the specific need (principle three) (pp. 179-180). In tested settings, conventional monitoring raised about 120 alarms where EarlySense raised about two, and EarlySense flagged most adverse events hours ahead. The lesson is not "no alerts". It is fewer, targeted alerts that arrive early and go to the one person who can act.

## 6. The challenges: privacy, automation, failure, exceptions (chapters 16 to 21)

**Change (chapter 16).** The book expects resistance from people who built careers on screens, and welcomes criticism as what sharpens the idea.

**Privacy (chapter 17).** No one reads terms and conditions. The research he cites puts the typical time spent at a second or two, and reading every privacy policy a person meets at weeks of work (pp. 189-194). He does not propose better legal text. He proposes:

- **Plain-language settings** beside each switch, as in Microsoft's early Cortana, rather than a cartoon mascot or all-caps legalese (pp. 194-195).
- **Collect only what the task needs.** A light that mimics sunrise needs the alarm time, not the contact list. He cites apps that were fined or mocked for taking the whole address book (p. 195).
- **Forget.** Drop detailed data once it has served its purpose, drop everything after long inactivity, let the person set a time limit on sharing (Glympse), and offer a "start over" control (p. 196).
- **Show the reasoning.** When the system concludes something about a person, list the observations that led there and let the person remove them (pp. 196-197). The screen exists here for advanced control, "not the experience".

**Automatic (chapter 18).** Fear of automation is correct, because automation is hard: it needs enough valid data to prove intent, confidence before acting, and awareness that a wrong guess can feel creepy (p. 199). When it works, it becomes boring and indispensable: automatic doors, crash-sensing air bags, automatic transmissions (pp. 199-201).

**Failure (chapter 19).** Worry about failure belongs late, when a design is readied for the real world, not in early ideas (p. 203). Two answers:

- **Sensors that prevent failure,** such as air bags that check a passenger's weight before firing (p. 204).
- **A screen as the backup,** in a deliberate "shift from primary to secondary". Nest keeps its dial for when habits change. Lockitron keeps a web control for giving a guest a key. TripIt lets a person fix an itinerary its automation got wrong (pp. 204-205).

**Exceptions (chapter 20).** Less is sometimes more, and sometimes not (p. 208):

- Watching a film for pleasure needs a screen.
- A major decision keeps an explicit act: a coffee can be paid for with no interface, but confirming a $200,000 home loan deserves "a button or two".
- Government decisions need accountable people making explicit choices.
- Things people love doing, like cooking, are not chores to remove.

**The future (chapter 21).** The book ends hoping its idea becomes boring and obvious, as automatic doors did.

## 7. Where the book is weak or dated

- **Its examples are 2014 products.** The book itself records that Square struggled with Auto Tab's adoption (chapter 9) and that Facebook acquired Moves (p. 101), and warns that some featured companies may be gone by the time a reader arrives (chapter 21). Treat every example as an illustration of a principle, never as a live precedent to point at.
- **The screen insomnia chapter rests on correlation.** Krishna calls the cancer link "not conclusive" himself (chapter 7). Nothing in the manifesto should lean on it.
- **One addiction story is contested inside the book.** The account that Facebook reverted a News Feed redesign because users left too quickly comes from a single writer's sources, and the book prints a Facebook designer's denial beside it (chapter 5). The chapter's wider argument, about products measured on time spent, rests on the analyst quotes, not on that story.
- **It was written for consumer hardware.** Most of its machine input comes from physical sensors. A record-keeping product has few of those. Its machine input is mostly the record itself: acts people have already taken, dates already stored, what the browser already knows, and other systems' data. That translation is the manifesto's to make, and is not in the book.
- **It does not discuss accessibility.** Work done out of sight still has to be perceivable when it matters, and its manual fallback still has to be usable by everyone.
- **Its enthusiasm for learning from personal data (chapter 14) sits in tension with its own privacy chapter (chapter 17).** The book resolves this with transparency and forgetting. For health-adjacent data about a third party, as a Practice holds about its Clients, the repo's own constraints go further than the book asks.
- **It says nothing about when a notification may be delayed or scheduled.** It argues for few, targeted, early alerts. The timing rules Doula Cloud follows come from its own ADRs, not from this book.
- **Its automation chapter is built from successes.** Automatic doors, air bags and transmissions all prove that automation can work. Clippy appears only in the chapter's subtitle. The book names the risks, but its evidence is survivorship.

## 8. Where Doula Cloud already meets the book

These are pointers to the records that decided each point, not claims that the book decided them.

- **Infer status from an act already done** — [ADR-0015](../adr/0015-three-facts-on-an-engagement-the-person-lives-in-the-login.md)'s third principle, and the Engagement that moves to `active` when its first Visit is scheduled ([#895](https://github.com/markgoho/doula-cloud/issues/895)). Nothing completes an Engagement automatically, which is the book's exceptions chapter applied to care that continues after a loss.
- **Derive rather than store or ask** — Visit type ([#281](https://github.com/markgoho/doula-cloud/issues/281)), overdue ([ADR-0038](../adr/0038-overdue-is-derived-and-notifies-nobody.md)), and Contract merge fields filled from the record ([#258](https://github.com/markgoho/doula-cloud/issues/258)).
- **No badge that follows a person around** — [ADR-0028](../adr/0028-the-shell-has-no-notification-bell.md).
- **An act starts a bounded message; a date passing alone does not** — [ADR-0035](../adr/0035-connecting-stripe-is-system-noticed-except-when-there-is-no-account.md) and ADR-0038. The one sweep with no starting act is the dormancy notice in `api/internal/billing/dormancy.go`, which New York's escheatment law requires.
- **Content-free notices** — [ADR-0009](../adr/0009-notification-is-one-term-two-voices-keyed-by-recipient.md) and [ADR-0030](../adr/0030-no-third-party-open-or-click-tracking-in-notification-email.md).
- **Collect only what the task needs, and forget** — an Offer shows a thin copy and stops serving Client details when it ends ([#230](https://github.com/markgoho/doula-cloud/issues/230)). Erasure redacts in place and shreds the key ([ADR-0027](../adr/0027-erasure-redacts-in-place-and-shreds-the-key.md)).
- **A manual path beside every automation** — the manual status move (ADR-0015) and manual Payment recording ([#271](https://github.com/markgoho/doula-cloud/issues/271)).
- **The software carries the weight for the doula** — the design brief's Tesler's Law row (`docs/design/brief.md`).
- **Open work this read-back produced** — ADR-0015's *is this care finished?* prompt ([#1262](https://github.com/markgoho/doula-cloud/issues/1262)), steps from goal on each journey map ([#1267](https://github.com/markgoho/doula-cloud/issues/1267)), an alert when an on-call coverage gap is saved with no cover ([#1093](https://github.com/markgoho/doula-cloud/issues/1093)), and filling in the browser's timezone for an Owner to confirm (a comment on [#1166](https://github.com/markgoho/doula-cloud/issues/1166)).
