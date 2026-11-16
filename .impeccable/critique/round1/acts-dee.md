Walk log: Dee Whitlock (persona-walker, blind), round 1 at 563c2f5c. Profile 1280x800x1, pointer. Shots shots/dee-act-<n>.png. 9 acts; nothing saved (no POST/PUT/DELETE after sign-in).

Act 1: sign in → Overview ("Who is free tonight": Camille Boyd under "Needs cover"; "Clients waiting on a reply": Camille Boyd).
Act 2 (goal 1): "Camille Boyd" → the record. Facts: "Owed $750.00, due Nov 29", "Contract Signed", "Next Visit Wed Nov 18, 11:00am", "On call tonight Nobody". Contract: "Signed on Sep 20, 2026. Sent on Sep 13, 2026 by Renata Alvarez." 2337 ms. Goal 1 answered with 0 actions on the record.
Act 3: the fact link "$750.00, due Nov 29" (name "…, Owed, in Invoices") → #invoices: "$750.00 owed, due Nov 29."; "Of the $1,500.00 Contract price, $1,500.00 is billed and $0.00 is not billed yet."; INV-0036 Oct 25, 2026, $750.00, Owed, "Record payment", "Void", "Write off"; INV-0001 Paid, "Paid Sep 21, 2026 by bank transfer", note "Deposit, bank transfer". The table has Sent, no Due column.
Act 4 (goal 2): "Record payment" → inline "Record a payment of $750.00": Method radios, Note (optional), Date received (11/15/2026 prefilled), Continue, Cancel.
Act 5: Bank transfer, a note, Continue → check step: Amount, Method, Note, Date received; "Confirm and record", "Change"; no Cancel on this step. Stopped. 2 actions to the form from the record (1 from the fact link).
Act 6 (goal 3): Jump to "Contract": "Signed on Sep 20, 2026. Sent on Sep 13, 2026 by Renata Alvarez." Goal 3 in 1 action.
Act 7: Activity "Contract and money": drafted Sep 11 3:00pm, priced, changed 3:12pm, sent Sep 13 10:30am (Renata Alvarez), signed Sep 20 7:48pm (Camille), Invoices and payment. GET activity?kind=money 200. URL stayed #contract (filter hrefs are #activity).
Act 8 (goal 4): Activity "Doulas and on call": "5 minutes ago Put on this work as the Doula: Bethany Kroll — Renata Alvarez" (and on-call changes, 5 minutes ago), "Sep 6, 2026, 11:20am Put on this work as the Doula: Priya Raman". GET activity?kind=doulas 200.
Act 9: hovered "5 minutes ago": no tooltip, no clock time.

Reached for and did not find: an exact time on today's Activity rows (relative only, no hover); a Cancel on the payment check step; a Due column on the Invoice row; the filter in the address.
