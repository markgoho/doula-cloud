/**
 * What the Settings index shows beside each setting (#2028): its current
 * value, in words, read through the same loaders each setting's own
 * screen uses. No endpoint of its own: every value is already one GET
 * away, and the reads run side by side, so the index waits for the
 * slowest of them rather than their sum.
 *
 * Each setting is drawn only for someone who may change it (#2028's
 * "a Doula sees only what she may change"), so `MAY_CHANGE` mirrors each
 * setting's write guard on the BFF -- the mount line named beside it.
 * Drawing only, like every predicate in `roles.ts`: the BFF is what
 * refuses the write. A value is read only for a setting the caller is
 * shown, so nobody's index asks for a read the BFF would refuse.
 */
import type { Fetcher } from './fetcher.js';
import { creditCount, loadBalance } from './billing.js';
import { loadTemplate as loadClientFieldTemplate } from './clientFieldTemplate.js';
import { loadContractTemplate } from './contractTemplate.js';
import { loadEmailSuppressions } from './emailSuppression.js';
import { formatAmount } from './invoice.js';
import { loadMfaRequirementImpact } from './mfaRequirement.js';
import { loadOnCallSettings, type OnCallSettings } from './onCall.js';
import { CONNECT_STATUS_BADGES, loadConnectStatus } from './payments.js';
import { loadTemplate as loadPlanTemplate, type Field as PlanField } from './planTemplate.js';
import { loadPracticeTimezone } from './practiceTimezone.js';
import { loadRates, type Rate } from './rates.js';
import { isOwner, isOwnerOrAdmin, type RoleSession } from './roles.js';
import { zoneLabel } from './timezones.js';
import { loadWebsite, type PracticeWebsite } from './website.js';

export type SettingKey =
	| 'name'
	| 'website'
	| 'timezone'
	| 'clientFields'
	| 'planTemplates'
	| 'contractTemplate'
	| 'onCall'
	| 'gettingPaid'
	| 'rates'
	| 'credits'
	| 'mfa'
	| 'blockedAddresses'
	| 'export'
	| 'delete';

type Session = Pick<RoleSession, 'roles'>;
const isAnyStaffMember = (): boolean => true;

/**
Who may change each setting: the role list on its write's mount line.
*/
export const MAY_CHANGE: Readonly<Record<SettingKey, (session: Session) => boolean>> = {
	// practicename/mount.go: OwnerOnly.
	name: isOwner,
	// website/mount.go: OwnerOnly.
	website: isOwner,
	// practicetimezone/mount.go: OwnerAndAdmin.
	timezone: isOwnerOrAdmin,
	// clientfieldtemplate/mount.go: OwnerAndAdmin.
	clientFields: isOwnerOrAdmin,
	// plans/mount.go: every Staff role (ADR-0008).
	planTemplates: isAnyStaffMember,
	// contracts/mount.go: OwnerOnly.
	contractTemplate: isOwner,
	// oncall/mount.go: OwnerAndAdmin.
	onCall: isOwnerOrAdmin,
	// payments/mount.go: payment terms are OwnerAndAdmin.
	gettingPaid: isOwnerOrAdmin,
	// practicerate/mount.go: OwnerAndAdmin.
	rates: isOwnerOrAdmin,
	// billing/mount.go: purchases are OwnerAndAdmin.
	credits: isOwnerOrAdmin,
	// staffauth/mount.go: OwnerOnly.
	mfa: isOwner,
	// mailsuppress/mount.go: OwnerAndAdmin.
	blockedAddresses: isOwnerOrAdmin,
	// The export and erasure are Owner-only (#288, #871).
	export: isOwner,
	delete: isOwner
};

/**
One setting's value. `href` makes the value a link: a web address.
*/
export interface SettingValue {
	text: string;
	href?: string;
}

/**
 * A value the index read, `'unavailable'` where its read failed (the row
 * says so rather than vanishing), and absent where the caller is not
 * shown the setting at all.
 */
export type SettingsSummary = Partial<Record<SettingKey, SettingValue | 'unavailable'>>;

function count(n: number, noun: string, plural = `${noun}s`): string {
	return `${n} ${n === 1 ? noun : plural}`;
}

/**
Where a Client and Stripe find the Practice, as the index says it.
*/
export function websiteValue(website: PracticeWebsite): SettingValue {
	if (website.mode === 'undeclared') return { text: 'Not answered yet' };
	if (website.mode === 'own') return { text: website.ownUrl, href: website.ownUrl };
	if (website.pageState === 'live') return { text: website.pageUrl, href: website.pageUrl };
	if (website.pageState === 'failed') return { text: 'Your page could not be loaded' };
	return { text: 'Your page is being published' };
}

/**
The questions a template asks: a section heading is not one.
*/
function questionCount(fields: readonly Pick<PlanField, 'type'>[]): number {
	return fields.filter((field) => field.type !== 'section_header').length;
}

export function questionsValue(n: number): string {
	return n === 0 ? 'No questions yet' : count(n, 'question');
}

export function planTemplatesValue(careFields: readonly PlanField[], birthFields: readonly PlanField[]): string {
	return `Care Plan: ${questionsValue(questionCount(careFields))}. Birth Plan: ${questionsValue(questionCount(birthFields))}.`;
}

export function onCallValue(settings: OnCallSettings): string {
	const start =
		settings.startRule === 'gestational_week'
			? `From week ${settings.startWeek}`
			: 'From the day a doula takes the birth';
	return `${start}, until ${count(settings.graceDays, 'day')} past the due date`;
}

const KIND_NAMES: Readonly<Record<Rate['kind'], string>> = { birth: 'Birth', postpartum: 'Postpartum' };

export function ratesValue(rates: readonly Rate[]): string {
	return rates
		.map((rate) => `${KIND_NAMES[rate.kind]}: ${rate.amountCents === null ? 'no rate' : formatAmount(rate.amountCents)}`)
		.join('. ');
}

export function blockedAddressesValue(n: number): string {
	return n === 0 ? 'None' : count(n, 'address', 'addresses');
}

async function read(load: () => Promise<SettingValue | string>): Promise<SettingValue | 'unavailable'> {
	try {
		const value = await load();
		return typeof value === 'string' ? { text: value } : value;
	} catch {
		return 'unavailable';
	}
}

/**
 * Every value the caller's index shows that has to be read. The Practice
 * name comes off the session and the export and deletion rows have no
 * value to read, so none of the three is here.
 */
export async function loadSettingsSummary(
	fetcher: Fetcher,
	practiceId: string,
	session: Session
): Promise<SettingsSummary> {
	const reads: Partial<Record<SettingKey, () => Promise<SettingValue | string>>> = {
		website: async () => websiteValue(await loadWebsite(fetcher, practiceId)),
		timezone: async () => {
			const { timezone } = await loadPracticeTimezone(fetcher, practiceId);
			return timezone === '' ? 'Not set' : zoneLabel(timezone);
		},
		clientFields: async () => {
			const { fields } = await loadClientFieldTemplate(fetcher, practiceId);
			return questionsValue(questionCount(fields.filter((field) => !field.archived)));
		},
		planTemplates: async () => {
			const [care, birth] = await Promise.all([
				loadPlanTemplate(fetcher, practiceId, 'care_plan'),
				loadPlanTemplate(fetcher, practiceId, 'birth_plan')
			]);
			return planTemplatesValue(care.fields, birth.fields);
		},
		contractTemplate: async () => {
			const { prose } = await loadContractTemplate(fetcher, practiceId);
			return prose.trim() === '' ? 'Not written yet' : 'Written';
		},
		onCall: async () => onCallValue(await loadOnCallSettings(fetcher, practiceId)),
		gettingPaid: async () => {
			const { status } = await loadConnectStatus(fetcher, practiceId);
			return CONNECT_STATUS_BADGES[status].label;
		},
		rates: async () => {
			const { rates } = await loadRates(fetcher, practiceId);
			return ratesValue(rates);
		},
		credits: async () => creditCount(await loadBalance(fetcher, practiceId)),
		mfa: async () => {
			const { required } = await loadMfaRequirementImpact(fetcher, practiceId);
			return required ? 'On for every Staff member' : 'Off';
		},
		blockedAddresses: async () => {
			const suppressions = await loadEmailSuppressions(fetcher, practiceId);
			return blockedAddressesValue(suppressions.length);
		}
	};

	const shown = (Object.keys(reads) as SettingKey[]).filter((key) => MAY_CHANGE[key](session));
	const values = await Promise.all(shown.map((key) => read(reads[key]!)));
	return Object.fromEntries(shown.map((key, index) => [key, values[index]]));
}
