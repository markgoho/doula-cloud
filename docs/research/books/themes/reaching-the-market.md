# Theme synthesis: reaching the market

## What this file is for

This is the second stage of ADR-0041 for one theme on the values track. It lays out what each of the nine values books claims about how a product like this one finds its first buyers, what a founder spends, where the founder shows up, and what a landing page says; where the books agree; where they conflict; and what the repo has already decided. It contains no ruling. Page citations follow each read-back's own page rule. The read-backs are in `docs/research/books/`.

## The question

Where does the founder go, what does the money buy, and does the January site have to be found or only arrived at?

## What each book claims

**Make: The Bootstrapper's Handbook** (Levels). Launch on Product Hunt, Hacker News, and Reddit, against the better point that a launch belongs where the customers are; reach the niche's trade magazine; build a personal press list (Launch, pp. 83, 99–101, 107). Keep launching; side-project marketing launches mini-apps as separate products (pp. 108–110; Grow, pp. 117–118). Grow organically; paid traffic only after traction at 100,000 monthly active users; do not hire growth hackers (Grow, pp. 115–117). Tell your own story and build in public; showing mistakes makes a founder look human (pp. 118–121; This book, p. 17). Fill Open Graph and Twitter Card tags and generate a share image per page (pp. 121–123).

**The Almanack of Naval Ravikant** (Jorgenson, 2020). Business networking is a complete waste of time; a maker of something interesting is found by the right people (Building Wealth, pp. 85–86). Learn both to sell and to build; selling includes marketing, recruiting, and PR (pp. 32, 63–64). Media is permissionless leverage; a brand is built on Twitter and YouTube and by giving away free work (pp. 35, 57, 60).

**The SaaS Playbook** (Walling, 2023). High-touch funnels need $500 a month or more in price; low-touch funnels are a volume game (Marketing, pp. 90–93). In-person events pay only if the return covers the cost: one enterprise deal may, twenty new small customers means raise prices or skip events (pp. 99–100). Score approaches on speed, cost, and scalability, prioritize with ICE, run one fast and one slow channel, keep a marketing changelog, measure every channel (pp. 102–106).

**Start Marketing the Day You Start Coding** (Walling, 2023 ed.). Traffic quality decides conversion; a mailing list converts up to ten times website traffic; measure every source and drop the ones that do not convert ("Nine Levels", pp. 18–21). An automated follow-up sequence converts a subscriber ("#1 Goal", pp. 27–29).

**Jobs To Be Done** (Ulwick, 2016). Buyers searching online rarely start with a product name; they enter a job step or a desired outcome, so campaigns and search are built on those words (ch. 4 §IX, pp. 115–116).

**Copyhackers: Uplift.** The One Reader and her stage of awareness decide the design and the copy's length; the more aware the reader, the less copy (Section 1, p. 11; Section 5, p. 48). A trusted-by row and social proof made a claim more believable; the losing page had no persuasive elements to build trust (Section 2, pp. 21–22). Voice-of-customer research, about the customer's needs rather than the product, is where the winning copy came from (Section 2, p. 20; Section 4, p. 35).

**Landing Page Hot Tips.** Social proof, founding year, usage figures, and the best testimonial above the fold (Tips #52, #82, #83; PDF pp. 100, 153, 154–155). A free teaser and no obstacles to a demo (Tips #40, #46; PDF pp. 83, 91). The first question a SaaS visitor asks is the price (Tip #85, PDF p. 157).

**Design for Cognitive Bias** (Thomas, 2020). No position on channels or spend. Its claims about what a page may do to a reader sit in the persuasion theme.

**The Best Interface Is No Interface** (Krishna, 2015). No position.

## Where the books agree

- Words before channels: Copyhackers, Hot Tips, and Ulwick all put the reader's own words and stage of awareness ahead of any channel choice; Walling's changelog and Levels's press list both start from a message that already exists.
- Measure every source and drop what does not convert: Walling in both books; Levels names the analytics tools.
- Organic before paid: Levels's 100,000-user threshold and Walling's funnel arithmetic both keep paid traffic out at this price and this stage.
- The founder is the voice: Levels's build-in-public, Naval's permissionless media, Walling's sell-and-build.

## Where the books conflict

- **Being in the room.** Naval calls networking a waste; Levels says launch where the customers are; Walling measures a show by whether its return covers its cost. Naval's claim is about the founder's own network, the other two about the buyer's venue, and none of the three says which a trade summit is.
- **Launch platforms.** Levels names Product Hunt and Hacker News and then concedes the better point against them in the same chapter.
- **What a list is for.** Walling's list is a funnel with a sequence; Naval's media is a brand with no funnel at all.
- **Evidence.** Walling's ten-times figure and Levels's thresholds are the authors' own experience, uncited; Copyhackers's results are one page each; Hot Tips is uncited throughout.

## What the repo already decided

- #991 (2026-09-08): founder voice, first person, on every channel through launch, matching the teaser's signed note; only platforms that will bring paying customers get an account and every other platform is ignored; no contact with DONA until the teaser is live; one pool of about $2,500 through January with the DONA fee inside it; launch-day targets of 150 confirmed subscribers and 5 pilot Practices, recorded as numbers to be wrong about, not promises; the pilot agency is the only real customer contact; a frameworks table assigns positioning, message, and channel plan to named books and lists Walling's SaaS Playbook among the books to consult, never structure.
- #992: the DONA summit's exhibitor terms, read first-hand: Bronze $400, Silver $750, Platinum $1,050, setup closes October 15, 2026, no refunds; the drafted contact email was discarded.
- #993: five venues worth a founder's time.
- #362 and #358: the social card carries a generated 1200×630 image, because a bare link reads as spam in a group.
- ADR-0014: the waitlist lives in Buttondown; one confirmation, one January broadcast.
- ADR-0016: a subscriber's channel rides on a hidden form field filled from `utm_source`; counts are a floor, never a total.
- #1264, a bottom-up funnel with every conversion rate labeled, was closed as not planned on 2026-09-10.
- Open: #998 (the booth), #999 (positioning and the primary buyer), #1001 (the one sentence), #1003 (the channel plan and what the $2,500 buys), #868 (the site's words), #284.
