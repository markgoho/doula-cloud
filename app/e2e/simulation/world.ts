// The seeded World description (#822, under map #759): a deterministic
// read of docs/simulation/worlds/rooted-birth-collective.md and
// docs/simulation/calendar.md into data #822's own tail provisioner
// consumes and #823's walk can check its own counts against. The named
// cast -- who exists, her role, her employment type -- is fixed by the
// World doc and never drawn from the seed; the seed governs only what
// the World doc leaves open: the anonymous tail's Client names, which
// month a birth due date falls in, and how the tail's total splits
// across the doulas who carry it.
//
// Two things this file does NOT produce, on purpose: an individual
// record for any of the 15 Rooted or 5 Okonkwo *walked* Clients (their
// names and details are #823's to create live, through the UI, as it
// walks each Persona's own book), and a provisioned Engagement for any
// contractor. `staffauth.AttachingWrite` (api/internal/staffauth/attach.go)
// only accrues an attachment for an employee Doula's own write; a
// contractor reaches an Engagement solely through a granted Offer, which
// the calendar itself reserves for the run to *walk* (month 2's first
// Offer to Fern Okada, the P3 probe naming Yolanda Prieto) rather than
// something day zero provisions. So Fern, Yolanda and Trish Halvorsen --
// whose own Attachment the World says has already ended -- carry no
// provisioned Engagement here; their live work, if any, arrives through
// #823's walk.
//
// A documented discrepancy, not resolved by further reading: the
// calendar's table says Rooted holds 58 Engagements, 15 walked and 43
// provisioned; its own prose footnote lists the walked ones as "Priya's
// four, Lena's three, Renata's own three, and Maya's five at Okonkwo" --
// which sums to 15 only by counting Maya's Okonkwo clients into a
// Rooted figure. Read literally, Rooted's own named walked total is 10
// (Priya + Lena + Renata), five short of the table's 15. This module
// builds to the table's totals for the record it writes (58 = 15 + 43)
// and provisions exactly 43; the remaining five walked Rooted
// Engagements are #823's to create as it walks, not predetermined here.

export type EngagementKind = 'birth' | 'postpartum';

export interface EmployeeDoula {
	slug: string;
	givenName: string;
	familyName: string;
	workState: string;
	// [min, max] Engagements she carries on day zero, calendar.md's
	// per-doula loads -- a fixed count is [n, n] (Rowan, Delia).
	range: [number, number];
}

// The eight employee Extras who carry live day-zero work. Excludes
// the three contractors (Fern, Yolanda, Trish) for the reason in the
// header comment above.
export const ROOTED_EMPLOYEE_EXTRAS: EmployeeDoula[] = [
	{ slug: 'joss', givenName: 'Joss', familyName: 'Adeyemi', workState: 'NY', range: [2, 5] },
	{ slug: 'marisol', givenName: 'Marisol', familyName: 'Terrazas', workState: 'NY', range: [2, 5] },
	{ slug: 'bethany', givenName: 'Bethany', familyName: 'Kroll', workState: 'NY', range: [2, 5] },
	{ slug: 'aditi', givenName: 'Aditi', familyName: 'Sundaram', workState: 'NY', range: [2, 5] },
	{ slug: 'charlene', givenName: 'Charlene', familyName: 'Boateng', workState: 'NY', range: [2, 5] },
	{ slug: 'rowan', givenName: 'Rowan', familyName: 'Petrosyan', workState: 'NY', range: [7, 7] },
	{ slug: 'delia', givenName: 'Delia', familyName: 'Marchetti', workState: 'NY', range: [3, 3] },
	{ slug: 'kimiko', givenName: 'Kimiko', familyName: 'Nakashima', workState: 'NY', range: [2, 5] }
];

export const ROOTED_TAIL_TOTAL = 43;

// Deterministic, seedable, dependency-free -- there is no seedable PRNG
// in this repo's Bun/Node stack, and this run's whole reproducibility
// claim rests on never reaching for Math.random(). Mulberry32 (Tommy
// Ettinger, public domain): a 32-bit state, one multiply-heavy mix per
// call, good enough statistical quality for "spread a few dozen due
// dates across six months," which is all this file ever asks of it.
// The `| 0`s below are int32 wraparound, not truncation: mulberry32's mix
// depends on the value overflowing exactly as a 32-bit int would, which
// Math.trunc does not reproduce.
export function mulberry32(seed: number): () => number {
	let a = seed >>> 0;
	return () => {
		// eslint-disable-next-line unicorn/prefer-math-trunc -- see above
		a = (a + 0x6D_2B_79_F5) | 0;
		let t = Math.imul(a ^ (a >>> 15), 1 | a);
		t ^= t + Math.imul(t ^ (t >>> 7), 61 | t);
		return ((t ^ (t >>> 14)) >>> 0) / 4_294_967_296;
	};
}

// Fisher-Yates, driven by the same seeded rng as everything else here --
// deterministic order for a given seed, uniform otherwise.
function shuffle<T>(rng: () => number, items: readonly T[]): T[] {
	const copy = [...items];
	for (let index = copy.length - 1; index > 0; index--) {
		const swapWith = Math.floor(rng() * (index + 1));
		const value = copy[index];
		copy[index] = copy[swapWith];
		copy[swapWith] = value;
	}
	return copy;
}

export interface DoulaAllocation {
	slug: string;
	provisionedCount: number;
}

// One round-robin pass: adds at most one to each doula under grace's cap,
// stopping as soon as remaining hits zero. Pulled out of distribute so
// its own loop never nests inside distribute's grace loop.
function fillOnePass(order: readonly EmployeeDoula[], counts: Map<string, number>, grace: number, remaining: number): number {
	let left = remaining;
	for (const doula of order) {
		if (left <= 0) return left;
		const cap = doula.range[1] + grace;
		const current = counts.get(doula.slug)!;
		if (current < cap) {
			counts.set(doula.slug, current + 1);
			left--;
		}
	}
	return left;
}

// Fills every doula to her range's minimum, then round-robins the
// remainder up toward her stated maximum. ROOTED_TAIL_TOTAL (43) exceeds
// what the eight employee Extras' stated "two to five" ranges can hold
// even at every doula's max (22 at minimum, 40 at maximum) once Rowan's
// fixed 7 and Delia's fixed 3 are subtracted out -- calendar.md calls
// its own per-doula figures estimates, not a hard ceiling, so once every
// doula has reached her stated max this raises the cap by one and keeps
// going, rather than distributing 43 unevenly to stay strictly inside a
// range the source document itself does not treat as fixed.
function distribute(rng: () => number, total: number, doulas: readonly EmployeeDoula[]): DoulaAllocation[] {
	const counts = new Map(doulas.map((doula) => [doula.slug, doula.range[0]] as const));
	let remaining = total - doulas.reduce((sum, doula) => sum + doula.range[0], 0);
	if (remaining < 0) {
		throw new Error(`world: total ${total} is less than the doulas' combined minimum`);
	}
	const order = shuffle(rng, doulas);
	for (let grace = 0; remaining > 0; grace++) {
		const before = remaining;
		remaining = fillOnePass(order, counts, grace, remaining);
		if (remaining === before && grace > 20) {
			throw new Error('world: distribute could not place the remaining total');
		}
	}
	return doulas.map((doula) => ({ slug: doula.slug, provisionedCount: counts.get(doula.slug)! }));
}

export interface SeededClient {
	slug: string;
	givenName: string;
	familyName: string;
	kind: EngagementKind;
	// YYYY-MM-DD. Empty for a postpartum Engagement, which the product's
	// own engagement_requests.due_date column leaves null for that kind.
	dueDate: string;
	assignedDoulaSlug: string;
}

export interface WorldSummary {
	rooted: { total: number; walked: number; provisioned: number };
	okonkwo: { total: number; walked: number; provisioned: number };
}

export interface StagedArrival {
	practiceSlug: 'ridgeline' | 'rooted' | 'okonkwo' | 'bell-ortiz';
	// Weeks after Rooted's own day zero (week 0). undefined: before day zero
	// (Ridgeline) or unscheduled -- Tasha's own choice (Bell & Ortiz).
	offsetWeeks: number | undefined;
	note: string;
}

// Pure data: #823's own orchestrator places a ScheduleStep against each
// offset. Ridgeline's stand-up itself is provision.ts's standUpRidgeline,
// deliberately not referenced here -- a function isn't JSON-serializable,
// and writeWorldRecord below writes this list out verbatim.
export const STAGED_ARRIVALS: StagedArrival[] = [
	{
		practiceSlug: 'ridgeline',
		offsetWeeks: undefined,
		note: 'Already on the product before day zero -- provision.ts\'s standUpRidgeline(), not walked, no log entry.'
	},
	{
		practiceSlug: 'rooted',
		offsetWeeks: 0,
		note: "The run's own day zero. Renata's signup is #823's first walked act; nothing is provisioned ahead of it."
	},
	{
		practiceSlug: 'okonkwo',
		offsetWeeks: 3,
		note: 'Maya signs up cold, entirely walked; nothing is provisioned for her Practice at all.'
	},
	{
		practiceSlug: 'bell-ortiz',
		offsetWeeks: undefined,
		note: "Tasha arrives mid-run at a point her own walk decides (calendar.md places it mid-month-3); not scheduled here."
	}
];

export interface WorldDescription {
	seed: number;
	tailClients: SeededClient[];
	allocations: DoulaAllocation[];
	summary: WorldSummary;
	stagedArrivals: StagedArrival[];
}

// Birth due dates by month -- calendar.md's own month counts: 4 in
// month 1 (already at term), 6, 6, 5, 4, 3, and 4 falling after the run
// ends (kept here as a seventh bucket so a tail birth Client can still
// land on one, since a live book on day zero ordinarily holds Clients
// whose due date outlives the run -- see worlds/rooted-birth-collective.md).
const BIRTH_MONTH_COUNTS = [4, 6, 6, 5, 4, 3, 4];
const RUN_START_UTC = '2027-01-04T00:00:00Z';

function monthDueDate(rng: () => number, monthIndex: number): string {
	const due = new Date(RUN_START_UTC);
	due.setUTCMonth(due.getUTCMonth() + monthIndex, 1 + Math.floor(rng() * 28));
	return due.toISOString().slice(0, 10);
}

// A weighted month draw from calendar.md's own counts. The exact kind
// split (birth vs. postpartum) and which individual tail Clients land
// in which month is this module's own estimate -- the calendar gives
// Rooted's overall 32-birth/26-postpartum split and its month-by-month
// birth counts, not a per-Client assignment, so a seeded draw is the
// closest reproducible reading of an already-estimated document.
function dueDate(rng: () => number): string {
	const total = BIRTH_MONTH_COUNTS.reduce((sum, count) => sum + count, 0);
	let roll = rng() * total;
	for (const [index, count] of BIRTH_MONTH_COUNTS.entries()) {
		if (roll < count) return monthDueDate(rng, index);
		roll -= count;
	}
	return monthDueDate(rng, BIRTH_MONTH_COUNTS.length - 1);
}

// A pool sized comfortably above ROOTED_TAIL_TOTAL (43) so a shuffle can
// hand out distinct pairs with no repeats; none overlaps a named cast
// member's given or family name, so a run log can never confuse a tail
// Client for one a Persona is walking.
const TAIL_GIVEN_NAMES = ['Morgan', 'Jamie', 'Casey', 'Skyler', 'Reese', 'Avery', 'Quinn', 'Harper'];
const TAIL_FAMILY_NAMES = ['Bishop', 'Calloway', 'Dunmore', 'Estrada', 'Falk', 'Grier'];

// describeWorld is this module's one export everything else exists to
// support: the same seed always returns a deep-equal description, and a
// different seed reshuffles the tail's names, kinds, due dates and
// per-doula split without ever touching a named cast member's own facts.
export function describeWorld(seed: number): WorldDescription {
	const rng = mulberry32(seed);
	const allocations = distribute(rng, ROOTED_TAIL_TOTAL, ROOTED_EMPLOYEE_EXTRAS);

	const namePool = shuffle(
		rng,
		TAIL_GIVEN_NAMES.flatMap((givenName) => TAIL_FAMILY_NAMES.map((familyName) => [givenName, familyName] as const))
	);

	const birthShare = 32 / 58; // Rooted's overall kind split, calendar.md's book table
	const tailClients: SeededClient[] = [];
	let nameIndex = 0;
	let clientNumber = 0;
	for (const allocation of allocations) {
		for (let index = 0; index < allocation.provisionedCount; index++) {
			clientNumber++;
			const [givenName, familyName] = namePool[nameIndex++];
			const kind: EngagementKind = rng() < birthShare ? 'birth' : 'postpartum';
			tailClients.push({
				slug: `tail-${clientNumber}`,
				givenName,
				familyName,
				kind,
				dueDate: kind === 'birth' ? dueDate(rng) : '',
				assignedDoulaSlug: allocation.slug
			});
		}
	}

	return {
		seed,
		tailClients,
		allocations,
		summary: {
			rooted: { total: 58, walked: 15, provisioned: ROOTED_TAIL_TOTAL },
			okonkwo: { total: 5, walked: 5, provisioned: 0 }
		},
		stagedArrivals: STAGED_ARRIVALS
	};
}
