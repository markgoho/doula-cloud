# Theme synthesis: what gets measured

## What this file is for

This is the second stage of ADR-0041 for one theme on the values track. It lays out what each of the nine values books claims about what a product like this one counts, how it attributes a customer to a source, and what it refuses to collect; where the books agree; where they conflict; and what the repo has already decided. It contains no ruling. Page citations follow each read-back's own page rule. The read-backs are in `docs/research/books/`.

## The question

What does the product count, what does it refuse to count, and what number stands in for churn when nobody subscribes?

## What each book claims

**The SaaS Playbook** (Walling, 2023). The 80/20 metrics chapter: monthly recurring revenue, churn, and the timed emails that move them (80/20 SaaS Metrics, pp. 151–161). Measure every marketing channel and keep a changelog (Marketing, pp. 102–106).

**Start Marketing the Day You Start Coding** (Walling, 2023 ed.). Measure every source with analytics goals and drop the ones that do not convert; a mailing list converts up to ten times website traffic ("Nine Levels", pp. 18–21). Ask customers how they found you and what they would use instead; last-click analytics under-credits the first touch; the benefit customers name may not be the one marketed ("Four Things I Learned", pp. 150–152).

**Make: The Bootstrapper's Handbook** (Levels). Google Analytics is good enough, Mixpanel is expensive, Amplitude is mostly free, and Hotjar's session streaming is useful though its ethics might be questionable (Launch, pp. 80–81).

**The Almanack of Naval Ravikant** (Jorgenson, 2020). Systems, not goals, citing Scott Adams; the author rejects self-measurement and says to help others without counting (Saving Yourself, pp. 183, 188; Building Wealth, p. 75).

**Design for Cognitive Bias** (Thomas, 2020). Clicks are easy to measure and may say nothing; there is close to an inverse relationship between how easy a metric is to measure and its value; Goodhart's Law (ch. 3, pp. 48–49). Whatever is incentivized gets gamed (ch. 3, pp. 41, 49).

**The Best Interface Is No Interface** (Krishna, 2015). Chapter 5 argues that engagement measured as screen time rewards the wrong thing, since the best interface is the one a person does not have to look at.

**Jobs To Be Done** (Ulwick, 2016). The outcome survey measures importance and satisfaction per desired outcome, on 180 to 3,000 respondents (ch. 4 §IV, p. 98; ch. 6, pp. 170–172). No position on product analytics.

**Landing Page Hot Tips.** Track marketing channels with platform-specific coupon codes (Tip #15, PDF p. 42). Its usage-figures tip (Tip #83, PDF pp. 154–155) is about what a page displays, carried by the persuasion theme.

**Copyhackers: Uplift.** No position on measurement beyond reporting each page's test result.

## Where the books agree

- Attribution by asking beats attribution by pixel: Walling's "ask how they found you", Hot Tips's per-channel coupon codes, and Thomas's "clicks may say nothing" point the same way from marketing and from ethics.
- A number made a target stops being a number: Thomas states Goodhart's Law; Naval's "systems, not goals" is the same claim in another register.
- Measure sources, not people: Walling measures channels; nothing in Levels, Walling, or Thomas measures an individual user's behavior except Levels's session streaming, which he himself flags.

## Where the books conflict

- **Whether to count at all.** Walling's metrics chapter is a dashboard; Naval refuses to keep one.
- **How much tooling.** Levels names four tools; Thomas warns that the easiest tool measures the least; Krishna's screen-time argument rejects the engagement metric those tools produce.
- **What replaces churn.** Every metric Walling names assumes a subscription; no book names the retention figure for a prepaid, per-unit product.
- **Evidence.** Walling's ten-times figure is uncited; Thomas cites Goodhart; Levels's tool opinions are his own.

## What the repo already decided

- ADR-0016: visitors are counted by Pirsch, cookieless by construction, under legitimate interest deliberately and not by default, with no consent banner; UTM parameters are a first-class dimension at venue granularity; a subscriber's channel is carried by four hidden form fields set once at signup and never joined to analytics after the fact; channel counts are a floor, never a total.
- ADR-0030: no Doula Cloud email carries open or click tracking, in either voice, because open tracking measures image loading, not reading, and nothing wants the data.
- #991: two launch-day targets, 150 confirmed subscribers and 5 pilot Practices, recorded as numbers to be wrong about, not promises.
- #45: Doula Cloud runs no recurring subscription; a Practice prepays per Engagement on a credit-balance model. No record names a retention figure for that model.
- #439: both Engagement kinds cost the same Credit, with the reasoning that a price varying by kind would invite a Practice to declare the cheaper one; the fairness argument for splitting them lost to the mis-declaration risk.
- #995 (2026-09-08): ten of eleven competing tools claim "one place" and every one sells a monthly per-seat subscription; none says how many doulas use it.
