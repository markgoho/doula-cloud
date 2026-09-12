# The Best Interface Is No Interface, read back

## 1. Source

*The Best Interface Is No Interface: The Simple Path to Brilliant Technology*, Golden Krishna, New Riders (an imprint of Peachpit/Pearson), 2015. ISBN 13: 978-0-133-89033-4. Read 2026-09-10 from the `pdftotext -layout` extraction of the 356-page PDF, front to back, with the PDF opened directly for the front-matter copyright page and two body pages where a figure needed checking.

**Page rule.** This PDF carries no printed page number on any page checked directly as an image (front matter, chapter openers, and mid-chapter pages alike), and the book's own back-of-book Index — which does carry the print edition's page numbers, e.g. "recycling bin, 40" and "Wood, Molly, 191" — does not translate onto this PDF by a fixed offset. Cross-checking four Index entries against where that content actually sits in this PDF gives four different offsets: "CNN headlines about apps, 6–7" sits at PDF page 78 (offset 72); "recycling bin, 40" sits at PDF page 112 (offset 72); "Cooper, Alan, 114" sits at PDF page 196 (offset 82); "Wood, Molly, 191" sits at PDF page 271 (offset 80). Because the offset drifts by ten pages across the book rather than holding constant, this file cites **the PDF page index**, alongside the book's own numbered chapter (1 through 21, each independently numbered and titled) since the book does number its chapters even though it does not print page numbers. A citation reads "(ch. 5, p. 132)".

## 2. What this file is for

This file is the evidence: what the book argues, page by page, with its own exceptions, what has changed since 2015, and where it is weak. It records no Doula Cloud decision; where the book touches something the repo has already decided, section 6 names the ADR or ticket, and the decision belongs to that record.

## 3. The claim

The book's thesis, stated as its own title and repeated at the close of chapter 1 and again as the heading of chapter 8, is that the best outcome for a technology product is no graphical screen at all — with a default of solving a problem by embracing a person's existing habits and by having a machine sense what it needs rather than asking, and a screen reserved for the case where those paths are exhausted (ch. 1, p. 76; ch. 8, p. 157). The book organizes this into three stated principles, each its own named part of the book: embrace typical processes instead of screens (Principle One, p. 157), leverage computers instead of serving them (Principle Two, p. 208), and adapt to individuals (Principle Three, p. 242). The audience is explicitly the maker, not the reader as a consumer: the closing line of the introduction frames the book as being for entrepreneurs, designers, and engineers (ch. 1, p. 76).

The author states what the claim is not, repeatedly and by name in chapter 20, titled "Exceptions." "The best interface is no interface" is presented not as a rule that no interface may ever exist, but as naming no interface as the best *possible* outcome, with insisting on it as an absolute called out as a mistake (ch. 20, p. 285). The book names its own carve-outs: entertainment consumed for pleasure, decisions large enough to want an explicit confirmation such as a major payment, and governmental or military decisions kept with an accountable person rather than handed to software making choices silently (ch. 20, p. 285).

## 4. The argument, chapter by chapter

### Foreword (Ellis Hamburger, unnumbered front matter)

Hamburger, then a reporter for The Verge, opens on why phones ring at all, and argues that Snapchat inverted the pattern by requiring both people to already be inside a chat window before video can start, rather than letting one person "ring" the other the way FaceTime does (unnumbered front matter). He credits Mark Weiser, Alan Cooper, and Don Norman with seeing the shift coming before he did, and closes on the idea that the next advance will come from forgetting what has been learned about interfaces and building around instinct instead (unnumbered front matter).

### Chapter 1: Introduction — Why Did You Buy This Book? (p. 75)

Two pages. States the book grew out of a widely shared essay and a conference keynote, quotes a hostile online comment calling the author a clown and agrees with part of it, and sets the book's tone of addressing its own critics directly rather than ignoring them (p. 75). Ends on the book's title line and frames what follows as an argument that love for the interface has become disturbing and is holding back real innovation (p. 76).

### Chapter 2: Screen-based Thinking — Let's Make an App! (pp. 77–98)

Opens with a montage of press coverage mocking the volume of trivial apps, including a curated list of real CNN headlines pairing mundane and absurd needs with "there's an app for that" (pp. 78–80), and a note that "app" has out-searched Justin Bieber and One Direction on Google Trends (p. 80). The chapter's central worked example is unlocking a BMW's doors by smartphone: thirteen steps from walking up to the car to the door opening, contrasted with a proximity-sensor system Siemens supplied to Mercedes-Benz more than a decade earlier that needs no screen at all, down to two steps (pp. 81–95). Edward Tufte's line that clutter and confusion are failures of design, not attributes of information, closes the comparison (p. 96).

### Chapter 3: Slap an Interface on It! (pp. 99–116)

A collage of roughly eighty real place names built on the "Silicon Valley" formula makes the point that imitation, not original thinking, dominates tech culture, alongside a Guy Kawasaki line urging founders to aim past recreating Silicon Valley rather than merely copying it (pp. 105–106). The chapter then runs a repeated "how do you make a better X" bit against a car, a refrigerator, a trash can, a restaurant, and a vending machine, with the same answer each time, "slap an interface on it," pointing to Tesla's dashboard touchscreen, a $47,000 LCD-screened London recycling bin, and touchscreen restaurant ordering as examples of adding a screen without asking whether one was needed (pp. 99–104).

### Chapter 4: UX ≠ UI — I Make Interfaces Because That's My Job, Bro (pp. 117–120)

A list of real job postings from Amazon, Apple, HP, Adobe, Dell, and others that all fuse "UX/UI" into one title is offered as evidence that the industry has collapsed two distinct disciplines into one, and that the fusion pushes designers toward producing more interface rather than better outcomes (p. 117). Two parallel lists define UI as navigation, buttons, and form fields, and UX as happiness, efficiency, and delight (p. 119), and the chapter closes on a Jeff Hammerbacher line about talented people spending their careers on ad-click optimization (p. 120).

### Chapter 5: Addiction UX (pp. 121–132)

Argues that a business model paid for by advertising eventually forces a product to maximize time-on-screen rather than to solve a person's problem efficiently, citing ad-revenue shares above 80 percent for Google, Facebook, Twitter, and Yahoo in the early 2010s (p. 125) and a Facebook engineer's account of a 2013 news-feed redesign that was reversed because it made people leave the site faster (pp. 130–131). Quotes Ethan Zuckerman's public apology for popularizing the pop-up ad and calling advertising the internet's founding mistake (p. 128), and closes with the author's own view that a designer's job is to take a person away from the technology, not to prolong the visit (p. 132).

### Chapter 6: Distraction (pp. 133–137)

Built around Clifford Nass's Stanford research on media multitaskers, whose 2013 study of 262 university students found that people who multitask most are worst at filtering out irrelevant information and at switching tasks (p. 134) — the opposite of what the researchers expected going in. The chapter's closing anecdote is Jason Humphreys, a Florida commuter fined $48,000 by the FCC for driving with an illegal cell-phone jammer for two years to stop himself and everyone near him from being distracted at highway speed (p. 136).

### Chapter 7: Screen Insomnia (pp. 138–146)

Argues that screens run at a color temperature that suppresses melatonin at night, citing a University of Basel study finding 40 percent less melatonin production under daylight-temperature light (p. 143) and a former Harvard Medical School dean's line that if light were a drug the government would not approve it (p. 142). Notes an FTC rule requiring lightbulb packaging to carry a Nutrition-Facts-style color-temperature label (p. 145), and reports the correlational finding — flagged by the author himself as not conclusive — that women in the brightest-lit neighborhoods showed a higher rate of breast cancer than those in darker ones (p. 140).

### Chapter 8: The Screenless Office (pp. 147–157)

Traces the "paperless office" prediction from a 1975 Businessweek feature and a Xerox PARC researcher's 1975 quote about calling up documents on a screen (p. 148), through a 1983 internal NSA newsletter mocking the whole idea as naive (pp. 149–151), to paper use actually falling after 2001 (p. 153, citing Paul Saffo's line that change is always slower than expected). Quotes Don Norman's 1990 complaint that an interface gets in the way of the job rather than helping with it (p. 155), and ends the book's opening section on the line "the best interface is no interface" as a design aspiration, not an absolute (p. 157).

### Chapter 9: Back Pocket Apps (pp. 158–192)

The longest chapter. Opens Principle One — embrace typical processes instead of screens — with phantom-vibration syndrome as a symptom of notification overload, then works through a series of case studies of apps that solve a problem without asking a person to look at a screen: the Moves fitness tracker, built by ProtoGeo and later bought by Facebook, that runs invisibly in a pocket (p. 183); Lockitron's second-generation deadbolt, which dropped its own hardware requirement and unlocks over Bluetooth without the phone leaving a pocket, after a first generation that needed thirteen steps almost identical to the BMW example (pp. 184–186); and the retreat of self-checkout lanes at chains including Albertsons after theft and confusion rose, quoted from Farhad Manjoo's argument that a human cashier remains faster and more pleasant than a machine (pp. 187–188). Closes on Square's Auto Tab, a Bluetooth-based payment feature that let a barista start a regular's order before they reached the counter, contrasted with MasterCard PayPass, Chase Blink, American Express ExpressPay, Visa payWave, Google Wallet, and Apple Pay, all of which still require pulling out a card or phone and waving it (pp. 189–191); Square's Auto Tab never reached the same feature at its biggest partner, Starbucks (p. 192).

### Chapter 10: Lazy Rectangles (pp. 193–207)

Argues that defaulting to a wireframe before understanding the problem produces generic screens rather than solutions, illustrated by two versions of one problem — a hot car — solved without any screen: Mazda's 1991 929, which used a solar-powered fan triggered by a temperature sensor, and Toyota's 2009 Prius, which improved on the same idea with a solar-powered moonroof vent (pp. 204–205).

### Chapter 11: Computer Tantrums (pp. 208–215)

Opens Principle Two — leverage computers instead of serving them — with the 1997 chess match in which IBM's Deep Blue beat Garry Kasparov, framed as proof computing power has vastly outpaced what most software actually asks of it (pp. 211–212), before pivoting to a real Windows error message demanding an 18,770-character password that never repeats any of the user's last 30,689 passwords (p. 208) as evidence that software design has not kept pace with hardware.

### Chapter 12: Machine Input (pp. 216–228)

Names Mark Weiser's 1991 Xerox PARC essay on ubiquitous computing as the intellectual root of the book's argument (p. 218), and traces the Active Badge project, an early 1990s infrared badge that let doors, phones, and displays react to a wearer's location without any typed input (pp. 220–221), as the ancestor of what the author calls machine input — sensed signals replacing typed form fields. Contemporary examples include a Petzl caving headlamp that senses ambient light and adjusts its beam automatically (pp. 222–223) and the Reebok Checklight, a sensor-based skullcap built with MC10 that signals possible head trauma with a traffic-light LED rather than a screen, discussed against the toll of football-linked chronic traumatic encephalopathy including the case of NFL linebacker Junior Seau (pp. 224–227).

### Chapter 13: Analog and Digital Chores (pp. 229–241)

Argues that busyness is chronic and casts software's role as absorbing chores rather than adding new ones, using Goodyear's self-inflating tire (which corrects pressure while driving with no app at all) and Whirlpool's sensor-driven washer and a "sensor cycle" dishwasher button that replaces a wheel of settings with one press (p. 236) as examples, and citing Barry Schwartz's finding that offering people thirty chocolates rather than six left them less satisfied, as evidence that more choice is not better design (p. 235). Closes on TripIt and IFTTT as early examples of software that removes a digital chore — an itinerary auto-built from a forwarded confirmation email — rather than adding one (pp. 239–240).

### Chapter 14: Computing for One — You're Spécial (pp. 242–252)

Opens Principle Three — adapt to individuals — arguing that most software is built for a statistical average and therefore serves nobody especially well, and that data science can instead adapt a product to a single person the way LinkedIn's Jonathan Goldman did in 2006 when he built the "People You May Know" feature against internal skepticism, which went on to drive a large share of the site's growth (p. 250).

### Chapter 15: Proactive Computing (pp. 253–263)

Opens on Elektro, a seven-foot voice-responsive robot Westinghouse built for the 1939 World's Fair, as an early and ultimately unrealized vision of talking to a computer (pp. 253–256), then argues the more useful target is not conversation but proactive anticipation: the Nest thermostat learning a household's temperature pattern well enough to stop asking (p. 258), and EarlySense, a mattress-embedded sensor that tracks a hospital patient's heart and respiratory rate and predicted adverse events hours ahead of standard monitoring while cutting false alarms from around 120 a day to about two (pp. 260–261). Closes on the NBA's SportVU camera system, adopted leaguewide after Kobe Bryant's 2013 Achilles injury, which lets a team's medical staff manage player workload without any wearable device (pp. 261–262).

### Chapter 16: Change — You Hate This Book? Thank You. (pp. 264–265)

A short, self-aware chapter acknowledging that people whose careers are built on screen-based design will dislike the book's argument, and welcoming the criticism as the mechanism by which a new idea gets tested (pp. 264–265).

### Chapter 17: Privacy (pp. 266–277)

Opens with the near-universal habit of clicking through legal agreements unread, citing a Carnegie Mellon estimate that reading every privacy policy an average American encounters in a year would take 76 working days, or about 54 billion hours nationally (pp. 268–269), and two experiments — PC Pitstop's hidden cash-prize clause and a UK retailer's soul-purchase clause in Gamestation's checkout terms — that both went almost entirely unread (p. 272). Argues the fix is not better legal writing but a transparent, plain-language settings menu, holding up an early Cortana privacy settings screen as the example worth copying (pp. 273–274), and Facebook's cartoon-dinosaur privacy explainer as the example of papering over a design problem instead (p. 270). Proposes that a system built to adapt to a person also forgets what it has learned about that person once it is no longer useful, or after a period of disuse (p. 276).

### Chapter 18: Automatic — Automatic Solutions Are Terrible. Look at Clippy! (pp. 278–280)

Argues that automatic solutions are genuinely difficult to get right and that people are correct to distrust them at first, but that a handful — automatic sliding doors dating to 1960, automatic transmissions from 1940 onward, and the airbag, standardized by US law in 1998 and credited with saving more than 10,000 lives — earned that trust over long periods and are now invisible parts of daily life (pp. 278–280).

### Chapter 19: Failure — What Happens When It All Falls Apart? (pp. 281–283)

Argues that any No-Interface system needs a considered answer for what happens when the automatic path fails, illustrated by automatic sliding doors that still offer a manual push-to-open option, and several of the book's own case studies — Nest, Lockitron, the Petzl headlamp, EarlySense, and TripIt — that all keep a screen-based interface as a secondary, backup path rather than the primary one (pp. 281–283).

### Chapter 20: Exceptions — Less Is Sometimes More (pp. 284–286)

States directly that "the best interface is no interface" is an aspiration, not an absolute rule, and names its own exceptions: entertainment consumed through a screen, a large one-time financial decision such as confirming a large home loan, and governmental or military decisions kept with an accountable person rather than automated away (p. 285).

### Chapter 21: The Future — Wow, This Is Boring (pp. 287–288)

Closes by hoping the book eventually reads as obvious and outdated, on the reasoning that if No-Interface thinking succeeds it will stop looking like an idea and start looking like the unremarkable, expected way things work — the way an automatic door or an airbag already does (pp. 287–288).

## 5. The book's own exceptions

These are the places the author limits his own claim, admits a counter-case, or says a rule does not apply.

- **The thesis itself is bounded, by title.** "The best interface is no interface" is stated as the best *possible* outcome, not the only acceptable one; treating it as an absolute would be a mistake the author names directly (ch. 20, p. 285).
- **Consumption entertainment is carved out.** Watching a movie or reading for pleasure is explicitly not a target for interface removal (ch. 20, p. 285).
- **Large one-time decisions keep an explicit control.** A major purchase, such as confirming a large home loan, is named as a case where a button or two is the right design, not a NoUI flow (ch. 20, p. 285).
- **Governmental and military decisions are excluded on principle**, not on difficulty — the author states plainly that automating a tax-policy or weapons-launch decision away from an accountable person is not the goal (ch. 20, p. 285).
- **Automatic solutions are conceded to be hard, and slow to trust.** The chapter on automation opens by validating the reader's own fear of automatic systems as reasonable, not irrational, before making the case that a handful — the automatic door, the automatic transmission, the airbag — took decades to earn that trust (ch. 18, pp. 278–280).
- **Every proactive or machine-input example in the book keeps a screen-based fallback.** Nest, Lockitron, the Petzl headlamp, EarlySense, and TripIt all retain a conventional interface as backup for when the automatic path is wrong or fails, and the author names this pattern directly as the sensible one (ch. 19, pp. 281–283).
- **Privacy-by-forgetting is conditional, not automatic charity.** The proposal that software forgets what it learns about a person is framed as a deliberate build choice for a company, contingent on the product's own trust-building goals, not a rule the author claims is already how most systems behave (ch. 17, p. 276).
- **The melatonin/cancer correlation is flagged by the author as not conclusive**, in his own words, immediately after presenting it (ch. 7, p. 140).
- **The book welcomes the possibility that it is wrong.** Chapter 16 is a standing invitation for the reader to argue back, on the reasoning that a counter-argument either strengthens the idea or exposes where it does not hold (ch. 16, pp. 264–265).

## 6. Where the book touches this repo

Each entry is the claim and its page, then the record that decided the matter or the ticket that holds it.

1. **The best outcome for a screen that asks a person for something is no screen at all** (ch. 1, p. 76; ch. 8, p. 157). Record: [ADR-0021](../../adr/0021-govuk-is-the-reference-for-service-patterns.md) and `docs/design/govuk-alignment.md`, which adopt the opposite default for exactly that moment — one question at a time, across as many pages as the questions need — naming the intake sequence and a three-box date of birth as work already built on it.
2. **UX and UI have been fused into one job title and one discipline, and the fusion is part of what pushes an industry toward producing more interface** (ch. 4, pp. 117, 119). Record: `docs/design/govuk-alignment.md` draws a structurally similar two-part line at the point GOV.UK enters the product — the decision and the behavior are adopted, the markup and the look are not — decided under ADR-0021.
3. **A designer's job is to take a person away from the interface, not to prolong the visit; an interface that distracts or nags is a defect** (ch. 5, p. 132; ch. 6, p. 137). Record: [ADR-0028](../../adr/0028-the-shell-has-no-notification-bell.md), which quotes the design brief's Goal-Gradient/Zeigarnik rule that unfinished work appears in a list a person reads when they choose to, never as a badge that pulses or a banner that follows a person around.
4. **Automatic solutions are genuinely hard to get right, and the ones that work earned trust over a long, careful track record** (ch. 18, pp. 278–280). Record: [ADR-0038](../../adr/0038-overdue-is-derived-and-notifies-nobody.md), which computes an Invoice's overdue state at read time rather than storing a flag or running a scheduled sweep, and [ADR-0035](../../adr/0035-connecting-stripe-is-system-noticed-except-when-there-is-no-account.md), which rejected a scheduled sweep for an unconnected Stripe account and requires a Staff member's own act instead.
5. **Sensed "machine input" is favored over typed input wherever a signal is available**, illustrated by an infrared location badge and a headlamp that reads ambient light on its own (ch. 12, pp. 220–222). Record: [#1166](https://github.com/markgoho/doula-cloud/issues/1166), which gives a Practice's calendar timezone a value a Staff member types and saves rather than one inferred from any signal, with its acceptance criteria checked closed against exactly that reading and change surface.
6. **A system reaching out proactively based on a person's inferred pattern, before being asked, is held up as the better model**, illustrated by a thermostat that stops asking and a hospital sensor that alerts hours ahead of a standard monitor (ch. 15, pp. 258, 260–261). Record: [#1266](https://github.com/markgoho/doula-cloud/issues/1266), which asked what a Practice with an untouched signup Credit grant hears after two years of silence, closed as not planned on 2026-09-10 — a ticket only, deciding nothing. ADR-0038 records that every outbox is nudged by an act and that a due date passing is not an act; `api/internal/billing/dormancy.go` holds a two-year dormancy notice whose comment cites New York's APL 1315(1-b) escheat deadline as the source of the interval.
7. **A transparent, plain-language settings screen is worth more than a legal document, illustrated by an early Cortana privacy-settings screen** (ch. 17, pp. 273–274). Record: none. No ADR or ticket decides what the signup form asks a person to consent to. `docs/research/practicelite-onboarding.md`, a research file and not a record, observes that the signup form collects no consent of any kind, against a named competitor's separate Terms/Privacy and SMS-consent checkboxes.

## 7. What has changed since publication

Each figure, tool, or company the book leans on, checked in September 2026 against a later source where one could be reached. "Unchecked" means this session could not reach a source that confirms or refutes the figure — this session's WebSearch budget was exhausted partway through checking, and several targeted WebFetch lookups at Wikipedia and company sites returned no relevant content, a 404, or a 403.

- **Nest, acquired by Google for $3.2 billion in 2014** (ch. 15, p. 258). Holds and grew: Google folded Nest into a unified "Google Nest" brand covering its whole smart-home line by 2019, absorbing what was previously Google Home hardware rather than the reverse.
- **TripIt, the email-forwarding itinerary builder** (ch. 13, p. 239). Still live and operating in 2026 under SAP Concur, which acquired TripIt's parent company in 2014; its site still advertises the same forward-a-confirmation-email mechanic the book describes, plus a paid Pro tier.
- **Cortana's privacy settings screen, held up as the model to copy** (ch. 17, pp. 273–274). The product itself no longer exists: Microsoft pulled the standalone Cortana app from iOS and Android in March 2021, deprecated the Windows app in August 2023, and removed Cortana from Windows 11 entirely in May 2024, replacing it with Microsoft Copilot. The example is now a screenshot of a discontinued product.
- **Kleiner Perkins Caufield & Byers (KPCB), cited for its smartphone-checking research** (ch. 9, p. 168). The firm has since shortened its public name to Kleiner Perkins, dropping "Caufield & Byers" from its trade name; the underlying legal entity is unchanged.
- **The self-checkout retreat the book describes at Albertsons and Big Y** (ch. 9, pp. 187–188). The trend continued and intensified rather than reversing: Target both limited how many items a shopper could ring up at self-checkout and closed some lanes entirely in 2023, against a reported 120 percent rise in retail theft that year, and the UK chain Booths eliminated self-checkout from its stores altogether in 2023, citing customer service rather than theft.
- **Lockitron, the second-generation Bluetooth deadbolt** (ch. 9, pp. 184–186; ch. 19, p. 283). `lockitron.com` no longer resolves as of this check (2026-09-10), which is consistent with the company having shut down, but this session found no dedicated source naming a closure date, so the fact of a shutdown is inferred from the dead domain rather than confirmed by a source describing it.
- **Square's Auto Tab and its Starbucks partnership** (ch. 9, pp. 189–192). Unchecked — no dedicated source was reached this session; a general Wikipedia article on Square, Inc. confirms the 2012 Starbucks partnership existed but says nothing about Auto Tab or when either arrangement ended.
- **EarlySense, the contactless hospital patient monitor** (ch. 15, pp. 260–261). Unchecked — the company's own site could not be reached and a Crunchbase lookup returned a 403.
- **MC10 and the Reebok Checklight** (ch. 12, pp. 224–227). Unchecked — no dedicated source was reached this session.
- **The Moves fitness-tracking app, built by ProtoGeo and acquired by Facebook** (ch. 9, p. 183). Unchecked — a Wikipedia lookup for the app returned no page.
- **Petzl's ambient-light-sensing headlamp** (ch. 12, pp. 222–223). Unchecked — Petzl's own site returned no readable content this session.
- **The Locket and KPCB phone-check-frequency figures, 110 and 150 times a day** (ch. 9, p. 168). Unchecked — neither figure was independently checked this session.

## 8. Where the book is weakest

- **Correlation presented as cause, flagged and then used anyway.** The Israeli research team's finding that women in brighter neighborhoods had a higher rate of breast cancer is introduced with the author's own caveat that the link is not conclusive, and then carried forward for several more pages as if it were closer to settled (ch. 7, pp. 140–142).
- **The book's own success stories are disproportionately short-lived.** Two of the strongest case studies for the value of NoUI thinking — Cortana's transparent settings menu, held up as the model worth copying, and Square's Auto Tab, the proximity-based payment feature — were both later discontinued by their own makers; the third-generation Lockitron deadbolt this session checked appears defunct as well. The book presents each as a durable proof of a better design philosophy rather than as one product decision inside a company that could reverse it.
- **Chapter 19, on failure, is thin next to the chapters selling automation.** The three chapters arguing for automatic and proactive solutions (11, 15, 18) run to roughly forty pages combined; the chapter devoted to what happens when an automatic system gets it wrong is three pages and names no case in which a shipped automatic feature actually failed and had to be walked back, even though the book's own bibliography includes at least one candidate in Square's Auto Tab.
- **Figures given without their own source, stated as settled.** The 76-working-day and 54-billion-hour privacy-policy figures are attributed to two named Carnegie Mellon researchers with no citation to the paper itself (ch. 17, pp. 268–269), and the "110 times a day" and "150 times a day" phone-check figures are each attributed to a company (Locket, then KPCB) whose own underlying methodology the book does not describe (ch. 9, p. 168).
- **An unlabeled collage used as evidence.** The roughly eighty-name "Silicon Valley" collage is sourced only to four outlets in a single caption — Wired, Inc., CNBC, and Wikipedia — with no indication of which name came from which source or when (ch. 3, p. 105).
- **A secondhand, unverifiable anecdote carries real weight.** The Camp Grounded story, used to argue that people are willing to pay to be separated from their phones, is relayed entirely through one unnamed conference acquaintance the author calls "Craig," with the author noting himself that he has no way to confirm the account (ch. 9, p. 182).
- **The author's own professional history sits inside the argument without being flagged at the point of use.** Alan Cooper is quoted approvingly on user research (p. 196) without noting, at that point in the text, that the author worked at Cooper, the design consultancy Alan Cooper founded — the affiliation is disclosed once, earlier in chapter 3, but not repeated where the quote appears (ch. 3, p. 104; ch. 12, p. 196).
