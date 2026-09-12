# Theme synthesis: founder time, hiring, funding

## What this file is for

This is the second stage of ADR-0041 for one theme on the values track. It lays out what each of the nine values books claims about working alone, the first hire, cofounders, outside money, and the expenses a founder forgets; where the books agree; where they conflict; and what the repo has already decided. It contains no ruling. Page citations follow each read-back's own page rule. The read-backs are in `docs/research/books/`.

## The question

Who works on Doula Cloud, who owns it, whose money is in it, and what changes at the first hire?

## What each book claims

**The SaaS Playbook** (Walling, 2023). Delegate roles, not tasks; support is usually the first hire; a founder who does not plan to hire is challenged to hire at least a support person (Team, pp. 117–126). Equity in an LLC passes a K-1 on paper profits; profit sharing suits a run-it-forever company; a cofounder is not needed and is the largest equity ever given away (Team, pp. 135–141).

**Make: The Bootstrapper's Handbook** (Levels). Work alone early because collaborations can be very dangerous; never pay a developer in equity for an idea, since an unexecuted idea's market value is zero (Idea, pp. 31–32; Build, p. 48). Build it yourself; outsourcing loses the race to the maker who fixes bugs over coffee; avoid hiring and build robots, because a hire makes the founder liable for their income; keep contractors on standby (Build, pp. 41–43; Automate, pp. 172, 177).

**The Almanack of Naval Ravikant** (Jorgenson, 2020). Labor is the worst form of leverage; code and media are permissionless and earn while you sleep (Building Wealth, pp. 34–35, 58, 60). Pick partners for intelligence, energy, and above all integrity; never partner with cynics (Building Wealth, p. 32; Learning Happiness, p. 148; Naval's Writing, p. 224). Own equity; capital is permissioned leverage that someone else has to grant (Building Wealth, pp. 31, 34–35, 53–54). Take business risks under your own name; avoid the risk of ruin (Building Wealth, pp. 34, 52, 66–67). Set an aspirational hourly rate and outsource anything that costs less (Building Wealth, pp. 36, 71–72).

**Start Marketing the Day You Start Coding** (Walling, 2023 ed.). The expenses a founder forgets: entity filing fees, an accountant, a lawyer, errors-and-omissions insurance, health and disability insurance, the employer half of FICA ("Expenses", pp. 81–83). Code, marketing, and money: a founder needs two of the three, and an idea is not one ("How to Recruit a Developer Entrepreneur", p. 155).

**Jobs To Be Done** (Ulwick, 2016). A small one-product company is told to hire the author's firm rather than run the method alone (ch. 7, pp. 177, 182, 186). No other position on staffing or funding.

**Design for Cognitive Bias** (Thomas, 2020). No position.

**Landing Page Hot Tips.** No position.

**Copyhackers: Uplift.** No position.

**The Best Interface Is No Interface** (Krishna, 2015). No position.

## Where the books agree

- No cofounder: Walling, Levels, and Naval all say so, for three different reasons (equity, danger, integrity).
- Own the equity and take no permissioned money early: Naval directly; Levels and Walling by silence on raising and by their bootstrapping premise.
- A hire is a liability before it is leverage: Levels's "liable for their income", Naval's "worst form of leverage", Walling's forgotten employer FICA.
- Outsource below a rate: Naval's hourly rate and Levels's contractors on standby.

## Where the books conflict

- **The first hire.** Walling says hire support, and challenges the founder who will not; Levels says avoid hiring and build robots; Naval says labor is the worst leverage. Walling's evidence is the founders he has advised; the other two argue from their own solo practice.
- **Build versus outsource.** Levels says build it yourself; Naval says outsource anything below your rate; Walling delegates roles.
- **Partners.** Naval spends three passages on choosing partners; Walling and Levels say to have none.

## What the repo already decided

- #375 (2026-08-28): a single-member New York LLC, a disregarded entity for federal tax; the S-corp election ruled out, not deferred, because the W-2 wage base is already full; an LLC, not a C-corp, with a C-corp only if the funding position changes; no cofounder taking equity; no employee on payroll before January; no outside investment before January; what changes when the first person is hired is parked under Not yet specified; one LLC holding several ventures, with Doula Cloud as a DBA under a neutral holding name (#376); the founder works the professional questions directly rather than routing them to a CPA and an attorney, and carries the verification burden himself.
- `docs/research/ny-llc-tax-obligations.md`: the obligations that switch on at the first hire, a research file and not a record.
- #29: solo dev, building with AI assistance.
- #375's decisions on #389 (unremitted sales tax passes through the LLC shield) and #383 (Doula Cloud never holds the money).
- ADR-0014 costed the waitlist in founder sessions rather than dollars.
- Open: #386, what insurance to carry and what event binds it.
