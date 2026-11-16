# Grader A, round 3 (Engagement record, live at 4b6f64aa)

## Expected and missing (written before reading the surface brief)

From the grader brief, the rubric's jobs, and one look at Camille's record at 1280x800:

1. Previous / next record without going back to the list (Linear issue j/k, Gmail older/newer, HubSpot record arrows) - not seen.
2. A header overflow menu ("...") holding the record's rarer acts (Stripe customer page Actions, Linear issue ... menu) - not seen; acts are spread in sections and in Details.
3. Copy link / copy ID for the record (Stripe customer page, Linear Cmd+Shift+C) - not seen.
4. Keyboard shortcuts for the record's main acts (Linear, Gmail r for reply) - not seen.
5. Status shown as a pill beside the title (Linear status, Stripe Active/Delinquent badge, HubSpot lifecycle stage) - care phase is plain text 'Care phase Active'.
6. Click-a-property-to-edit in the side panel (Linear properties panel, Notion) - facts column is links to sections; edits live in Details via separate buttons.
7. The Client's address / where the birth is (Jane App patient profile, Practice Better client profile) - not at a glance; Details shows email only.
8. Files / attachments on the record (HubSpot attachments, Jane documents, Practice Better documents) - not seen.
9. A sticky header or sticky side facts while scrolling a long record (Stripe customer page, Linear sidebar) - to check.
10. Print / export the whole record (Jane chart print) - only the Birth Plan prints.
11. Internal staff note / comment thread distinct from Client messages (HubSpot notes, Linear comments) - Care Plan stands in, not a thread.

### Against the brief's Category baseline (read after the list above)

| # | Expected | Baseline | Verdict |
|---|---|---|---|
| 1 | Next/previous record | row 17 Declined, "no job on record; Back returns to the list" | Holds weakly: Renata working down "Clients waiting on a reply" would use it; Back + #2949 (return to the row) is an acceptable substitute pre-launch. |
| 2 | Overflow "..." menu | row 28 Declined (one header move, acts beside their fact, GOV.UK has no overflow pattern) | Holds for a Stripe/Linear-trained reader: every rarer act was findable in its section in 1 Jump-to press. |
| 3 | Copy link / ID | row 15 Declined, "address bar holds it" | Holds. |
| 4 | Keyboard shortcuts | row 16 Later (#2956) | Deferred to an app-wide decision; holds. |
| 5 | Status pill by title | row 1 Build (as text) + row 12 | Built as text "Care phase Active"; only nobody-on-call is a Badge. Acceptable (One Meaning Per Hue). |
| 6 | Edit from the property | row 11 Have it (each row's Change) | Built in Details (Change, Change the due date/care phase/Doulas). Holds. |
| 7 | Client's address | not in the baseline (row 27 is phone + email only) | MISSING. GLOSSARY.md:29 puts home address in the Client's structural core; Jane / Practice Better show it on the record; a doula driving to a home Visit or a labor at home ("At home. I will come to you first", Camille's thread) needs it. Counts toward 4e. |
| 8 | Files / attachments | not in the baseline | No job on record I can cite; noted, not failed. |
| 9 | Sticky header/facts | row 7 Declined (GOV.UK sticky-element zoom/focus faults) + Back to top | Holds. |
| 10 | Print whole record | row 18 Declined; Birth Plan prints | Holds. |
| 11 | Internal staff notes | row 29 Build (Care Plan, Visit notes) | Built. |

## Live notes (page 31, marked window.__grader='A')

- 1280x800 (A-camille-1280.png): header = back link "Clients", h1, "Message the Client", Care phase Active, Due Dec 6, 2026, 37 weeks, Doulas Priya Raman, Bethany Kroll, amber Badge "Nobody on call tonight". Facts column beside the main column: On call tonight Nobody / Next Visit Thu Nov 19, 11:00am Priya Raman / Contract Signed / Owed $750.00, due Nov 29 / Messages (warning icon) Unanswered since Thursday, 9:22am / Phone (585) 555-0142 (bottom ~785). On call h2 at 483, state line, first row (Priya) ~700 < 800. Facts column's top rule at ~383px vs "Jump to" text at ~396: not on one line.
- Console: no errors or warnings on load (1e pass).
- "Engagement": 0 in visible text, 0 in aria-labels.
- 1440x900: page 5158px (<= 5400); scrollWidth 1430; main column 36-1060; Visits table 903 (max-content 1805), Invoices 903 (max-content 1098), Activity 651 (= max-content); Messages content right edge 693, Offers 720, Birth outcome 693 (prose at --measure 65ch); facts column 1102-1374; one "Back to top" at the page end, no section taller than 900px (tallest On call 630). Birth Plan section 527px tall.
- 375x740: scrollWidth 365; "Birth Plan" link 445-470; On call tonight fact bottom 612; Phone fact top 798 (below the screen; act log says 741 at DPR 2). Screenshot at 375 timed out twice; used shots/priya-r3-act-2.png.
- T10, by refusing one fetch on my own page (initScript, nothing written): Invoices -> "We could not load the Invoices." + "Try again", Owed fact "Could not be read", other sections stood. Visits -> "We could not load the Visits." + "Try again", Next Visit fact "Could not be read". Engagement's own read refused: unverified.
- Imani Brooks (1280): Owed "$950.00 overdue since Nov 3"; no weeks; no On call tonight fact (postpartum); Phone "Not given"; order Visits, Invoices, Birth outcome, Contract, Care Plan, Messages ("No Messages yet"), Birth Plan, Offers, On call, Activity.
- A11y tree: h1 Camille Boyd; header facts follow with labels; h2s On call, Birth Plan, Visits, Messages, Care Plan, Contract, Invoices, Offers, Birth outcome, Activity in drawn order. "Details" (a Jump to target) is a complementary landmark whose title is plain text, not a heading: heading navigation never reaches it. Phone link name "(585) 555-0142, Phone, call": names the number, not that it calls the Client.
- Money: employee Doula reads money (ADR-0008, #282 amendment), so Priya's Owed/fee/Invoices pass 5b; money writes gated at InvoiceSection.svelte:142.
- Clients list rows link to the Client page, not the record (one extra hop to the record from the list; T1 starts "on arriving").

## Scores

See the report returned to the orchestrator (identical): Usability 38/40, Craft 19/20.
Failed: 4e (Client's address missing, my vote), 4f (T8: Details not a heading; Phone link does not say it calls the Client), C4 (facts column top not level with Jump to, ~13px).

