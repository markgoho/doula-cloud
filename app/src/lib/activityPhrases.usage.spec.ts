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

/** Every `ActionX ... = "value"` constant in the file, whether or not it
 * names a type: `ActionInvoiceRaised EngagementAction = "invoice_raised"`
 * and the untyped `ActionInvoiceRaised = "invoice_raised"` both match,
 * and the type is captured when present.
 *
 * Matching the untyped form matters, and is the difference between a gate
 * and a comfort: inside a `const (...)` block Go accepts a bare
 * `ActionFoo = "foo"`, whose untyped-string constant still converts
 * implicitly at every `activity.Record` call and in the `staffingActions`
 * map literal. A guard that only saw the typed form would let such an
 * action ship unphrased and throw in a Client's own render -- the exact
 * failure CONTEXT.md's Activity entry now says cannot happen. So an
 * untyped one is a loud failure here rather than an invisible one. */
const ACTION_CONSTANT = /^\s*(Action[A-Za-z0-9]+)(?:\s+([A-Za-z]+))?\s*=\s*"([a-z0-9_]+)"/gm;

/** The `staffingActions` map literal's own body, and the `ActionX: true`
 * entries inside it. Bounded by the closing brace at column zero, so the
 * `moneyActions` map above it is never swallowed. */
const STAFFING_MAP = /var staffingActions = map\[EngagementAction\]bool\{([\s\S]*?)\n\}/;
const MAP_ENTRY = /\b(Action[A-Za-z0-9]+):\s*true/g;

interface ActionConstant {
	name: string;
	/**
	 * The declared type, or `undefined` where the line names none.
	 */
	type?: string;
	value: string;
}

function actionConstants(): ActionConstant[] {
	return source
		.matchAll(ACTION_CONSTANT)
		.map((match) => ({ name: match[1]!, type: match[2], value: match[3]! }))
		.toArray();
}

/**
 * Every action a write site can record against an Engagement.
 */
function engagementActions(): string[] {
	return actionConstants()
		.filter((constant) => constant.type === 'EngagementAction')
		.map((constant) => constant.value);
}

/** The staffing set, resolved from constant *names* back to their string
 * values -- the map literal spells `ActionOfferSent`, not
 * `"offer_sent"`, so a name-to-value index built from the same file is
 * what turns one into the other. */
function staffingActionValues(): string[] {
	const byName = new Map(actionConstants().map((constant) => [constant.name, constant.value]));
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

	it('refuses an action constant that declares no type', () => {
		// The silent-miss this gate would otherwise have: an untyped
		// `ActionFoo = "foo"` inside the const block still works everywhere
		// the typed form does, so leaving it out of the vocabulary read
		// above would quietly exempt it from needing a phrase. Named here
		// so the failure says what to do -- give the constant its type --
		// rather than surfacing later as a Client's blank ledger.
		const untyped = actionConstants()
			.filter((constant) => constant.type === undefined)
			.map((constant) => `${constant.name} = "${constant.value}"`);

		expect(untyped, 'name the constant\'s type so the register knows whether it needs a phrase').toEqual([]);
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

	it('still has contract_void_requested as its longest phrase', () => {
		// The Client-portal hub's own 320px sweep opens the disclosure on a
		// `contract_void_requested` row specifically, because that is the
		// widest string this column can be asked to lay out (ADR-0024/0025).
		// A longer phrase added here would quietly demote that sweep to
		// measuring an average case, so the choice is pinned rather than
		// left as a hand count in a comment.
		const byWidth = clientReachableActions().toSorted(
			(a, b) => clientActivityPhrase(b).length - clientActivityPhrase(a).length
		);

		expect(byWidth[0]).toBe('contract_void_requested');
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
			'clientRegister: no Client wording for activity action "some_new_action"'
		);
	});
});
