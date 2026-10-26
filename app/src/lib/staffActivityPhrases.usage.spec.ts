import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

import { describeActivityAction } from './activityLedger.js';
import { staffActivityPhrase, staffActivityPhrasedActions } from './staffActivityPhrases.js';

/*
 * #2006's drift guard, the Staff twin of `activityPhrases.usage.spec.ts`:
 * every action a write site can record must have Staff words, and no
 * phrase may outlive its action.
 *
 * It reads two Go files. `api/internal/activity/actions.go` names every
 * typed action -- Engagement, Membership, Client and Practice -- once, by
 * gofmt's own one-constant-per-line layout. `api/internal/client/events.go`
 * names a Client record's own five events, which are that package's
 * sealed vocabulary (ADR-0027) and not in actions.go.
 */

const appRoot = fileURLToPath(new URL('../../', import.meta.url));
const repoRoot = path.join(appRoot, '..');
const actionsGo = readFileSync(path.join(repoRoot, 'api', 'internal', 'activity', 'actions.go'), 'utf8');
const clientEventsGo = readFileSync(path.join(repoRoot, 'api', 'internal', 'client', 'events.go'), 'utf8');

const TYPED_ACTION = /^\s*(?:const\s+)?Action[A-Za-z0-9]+\s+(EngagementAction|MembershipAction|ClientAction|PracticeAction)\s*=\s*"([a-z0-9_]+)"/gm;
const CLIENT_EVENT = /^\s*event[A-Za-z]+\s+eventType\s*=\s*"([a-z0-9_]+)"/gm;

function writeSideActions(): string[] {
	const typed = actionsGo.matchAll(TYPED_ACTION).map((match) => match[2]!);
	const clientEvents = clientEventsGo.matchAll(CLIENT_EVENT).map((match) => match[1]!);
	return [...new Set([...typed, ...clientEvents])].toSorted((a, b) => a.localeCompare(b));
}

describe('the Staff ledger has words for every action a write site records (#2006)', () => {
	it('reads a real vocabulary from each Go file', () => {
		// A regex that matched nothing would compare two empty sets.
		expect(actionsGo.matchAll(TYPED_ACTION).toArray().length).toBeGreaterThan(60);
		expect(clientEventsGo.matchAll(CLIENT_EVENT).toArray()).toHaveLength(5);
	});

	it('phrases exactly the write side, no more and no less', () => {
		expect(staffActivityPhrasedActions()).toEqual(writeSideActions());
	});

	it('never prints a write-side word a person would not say', () => {
		// The words #2006 found on screen: the system's names for its own
		// records and states, which the humanized action string carried.
		const systemWords = /\b(instance|priced|provisioned|superseded|engagement|portal|attached|logged|raised)\b/i;
		const offenses = writeSideActions()
			.map((action) => staffActivityPhrase(action, 'Amara'))
			.filter((phrase) => systemWords.test(phrase));

		expect(offenses).toEqual([]);
	});

	it('names the subject wherever the humanized action would not', () => {
		// "Payment terms changed" is already a person's words; "Created" is
		// not, because it does not say what was created or for whom.
		const unchanged = writeSideActions().filter(
			(action) => staffActivityPhrase(action, 'Amara') === describeActivityAction(action)
		);

		expect(unchanged).toEqual(['payment_terms_changed', 'practice_name_changed']);
	});
});
