import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

import { describeActivityAction } from './activityLedger.js';
import { clientActivityPhrase, clientActivityPhrasedActions } from './clientRegister.js';

/*
 * #708's drift guard, in the mold of `spelling.usage.spec.ts` (#899/#921)
 * and `clientRegister.usage.spec.ts` (#212/#834): a usage gate that reads
 * something outside `app/src` and fails when this side stops matching it.
 *
 * What it reads is the write side's own action vocabulary --
 * `api/internal/activity/actions.go`, where every `EngagementAction` is
 * named exactly once and `staffingActions` names the subset the Client
 * portal's reader excludes in SQL. The difference between those two sets
 * is precisely the set of actions a Client can meet, and the Client
 * register has to hold a phrase for every one of them.
 *
 * `ClientAction` (`portal_sign_in_address_changed`,
 * `portal_sessions_ended`) is deliberately not read: those are recorded
 * against `SubjectClient`, and the portal's ledger reads
 * `SubjectEngagement` only, so no such row ever reaches this table. The
 * same goes for `client/events.go`'s own sealed event types, which are
 * that package's vocabulary rather than this one.
 *
 * The check is exact set equality in both directions, mirroring
 * `TestStaffingActions_ContainsExactlyTheRosterSet` on the Go side: a new
 * write-side action with no phrase fails, and so does a phrase left
 * behind by an action the write side removed. Lexical, like every other
 * usage gate here -- parsing Go properly would buy nothing, since the
 * constant block is one declaration per line by gofmt's own doing.
 */

const appRoot = fileURLToPath(new URL('../../', import.meta.url));
const repoRoot = path.join(appRoot, '..');
const actionsGo = path.join(repoRoot, 'api', 'internal', 'activity', 'actions.go');

const source = readFileSync(actionsGo, 'utf8');

/** `ActionInvoiceRaised EngagementAction = "invoice_raised"` -- the typed
 * form each constant takes on its first (and, inside a `const (...)`
 * block, only) line. */
const ENGAGEMENT_ACTION = /\bEngagementAction\s*=\s*"([a-z0-9_]+)"/g;

/** The `staffingActions` map literal's own body, and the `ActionX: true`
 * entries inside it. Bounded by the closing brace at column zero, so the
 * `moneyActions` map above it is never swallowed. */
const STAFFING_MAP = /var staffingActions = map\[EngagementAction\]bool\{([\s\S]*?)\n\}/;
const MAP_ENTRY = /\b(Action[A-Za-z0-9]+):\s*true/g;

function engagementActions(): string[] {
	return source.matchAll(ENGAGEMENT_ACTION).map((match) => match[1]!).toArray();
}

/** The staffing set, resolved from constant *names* back to their string
 * values -- the map literal spells `ActionOfferSent`, not
 * `"offer_sent"`, so a name-to-value index built from the same file is
 * what turns one into the other. */
function staffingActionValues(): string[] {
	const byName = new Map<string, string>();
	for (const match of source.matchAll(/\b(Action[A-Za-z0-9]+)\s+EngagementAction\s*=\s*"([a-z0-9_]+)"/g)) {
		byName.set(match[1]!, match[2]!);
	}
	const block = STAFFING_MAP.exec(source);
	expect(block, `no staffingActions map literal in ${actionsGo}`).not.toBeNull();
	return block![1]!
		.matchAll(MAP_ENTRY)
		.map((match) => {
			const value = byName.get(match[1]!);
			expect(value, `staffingActions names ${match[1]}, which is not an EngagementAction`).toBeDefined();
			return value!;
		})
		.toArray();
}

function clientReachableActions(): string[] {
	const staffing = new Set(staffingActionValues());
	return engagementActions()
		.filter((action) => !staffing.has(action))
		.toSorted((a, b) => a.localeCompare(b));
}

describe("the Client register holds every action a Client can reach (#708)", () => {
	it('reads a real action vocabulary from the Go source', () => {
		// A regex that silently matched nothing would make every assertion
		// below pass while comparing two empty sets.
		expect(engagementActions().length).toBeGreaterThan(20);
		expect(staffingActionValues().length).toBeGreaterThan(0);
	});

	it('phrases exactly the Client-reachable set, no more and no less', () => {
		expect(clientActivityPhrasedActions()).toEqual(clientReachableActions());
	});

	it('never phrases a staffing action the portal reader already excludes', () => {
		const phrased = new Set(clientActivityPhrasedActions());

		expect(staffingActionValues().filter((action) => phrased.has(action))).toEqual([]);
	});

	it('says something other than the staff humanizer for every action', () => {
		// The defect itself, stated directly: a phrase that happens to
		// equal `describeActivityAction`'s output is the raw domain word
		// with its underscores taken out, which is what #708 is about.
		const jargon = clientReachableActions().filter(
			(action) => clientActivityPhrase(action) === describeActivityAction(action)
		);

		expect(jargon).toEqual([]);
	});

	it('never puts a team noun or a raw status value in a phrase', () => {
		// The same two bans `clientRegister.usage.spec.ts` holds over
		// `routes/portal/**`, applied to the phrases themselves -- that
		// gate walks `.svelte` files only, so a phrase written here would
		// slip past it entirely.
		const banned = /\b(Engagement|Plan Instance|Portal Account|Offer|Membership|Attachment)\b/;
		const offenses = clientReachableActions()
			.map((action) => [action, clientActivityPhrase(action)] as const)
			.filter(([, phrase]) => banned.test(phrase));

		expect(offenses).toEqual([]);
	});
});

describe('clientActivityPhrase', () => {
	it('refuses an action the register does not hold, rather than printing it', () => {
		expect(() => clientActivityPhrase('some_new_action')).toThrow(
			'clientRegister: no Client phrase for activity action "some_new_action"'
		);
	});
});
