// PROTOTYPE -- throwaway, wayfinder ticket #1502. Copy and host content for
// the pilot banner and the feedback page, per shell. Nothing here is
// production code.

export type Shell = 'staff' | 'portal';
export type Kind = 'broken' | 'idea' | 'other';

export const PRACTICE_NAME = 'Finger Lakes Birth Collective';

// Charting Q5: one required radio group; each choice becomes a label on the
// private issue.
export const kinds: readonly { value: Kind; label: string }[] = [
	{ value: 'broken', label: 'Something is not working' },
	{ value: 'idea', label: 'An idea or a request' },
	{ value: 'other', label: 'Something else' },
];

// A host screen the person is on when they decide to send Feedback. `path`
// is a real app path, so `match()` from `$app/paths` resolves it to the
// real route id -- the route pattern that reaches GitHub (#1501 Q1).
export interface HostScreen {
	key: string;
	title: string;
	path: string;
}

export const screens: Record<Shell, readonly HostScreen[]> = {
	staff: [
		{ key: 'clients', title: 'Clients', path: '/practices/p_7f3a/clients' },
		{
			key: 'client',
			title: 'Alex Rivera',
			path: '/practices/p_7f3a/clients/c_91b2',
		},
	],
	portal: [
		{ key: 'care', title: 'Your care', path: '/portal/engagements/e_55c0' },
		{
			key: 'contract',
			title: 'Contract',
			path: '/portal/engagements/e_55c0/contract',
		},
	],
};

interface Copy {
	bannerText: string;
	bannerLink: string;
	pageTitle: string;
	intro: string;
	legend: string;
	textLabel: string;
	textHint: string;
	destination: string;
	role: string;
	email: string;
	confirmationTitle: string;
	confirmationBody: string;
}

// ADR-0047: say what it is and why it is there. ADR-0021: no "please",
// "valid", "invalid" or "required" in an error.
export const copy: Record<Shell, Copy> = {
	staff: {
		bannerText: 'Doula Cloud is new, and you are one of the first to use it.',
		bannerLink: 'Tell us what is not working or what you need',
		pageTitle: 'Send feedback to Doula Cloud',
		intro:
			'The Doula Cloud team reads every piece of feedback during the pilot. It is how we decide what to fix first.',
		legend: 'What kind of feedback is it?',
		textLabel: 'Tell us more',
		textHint: 'What were you trying to do, and what happened?',
		destination: 'This goes to the Doula Cloud team, not to your Practice.',
		role: 'Owner',
		email: 'jordan@fingerlakesbirth.example',
		confirmationTitle: 'Feedback sent',
		confirmationBody:
			'Thank you. If a reply would help, Mark Goho, who builds Doula Cloud, will email you at jordan@fingerlakesbirth.example.',
	},
	portal: {
		bannerText: 'This care portal is new.',
		bannerLink: 'Tell us what is not working or what you need',
		pageTitle: 'Send feedback about this portal',
		intro: `${PRACTICE_NAME} uses Doula Cloud to run this portal. The Doula Cloud team reads every piece of feedback.`,
		legend: 'What kind of feedback is it?',
		textLabel: 'Tell us more',
		textHint: 'What were you trying to do, and what happened?',
		destination: `This goes to the Doula Cloud team, not to ${PRACTICE_NAME}. For anything about your care, message your doula.`,
		role: 'Client',
		email: 'alex.rivera@example.com',
		confirmationTitle: 'Feedback sent',
		confirmationBody:
			'Thank you. If a reply would help, the Doula Cloud team will email you at alex.rivera@example.com.',
	},
};

export const ERROR_KIND = 'Select what kind of feedback it is';

// Charting Q4: what the app attaches, listed on the form so nothing is
// collected silently (ADR-0046).
export interface Context {
	screenTitle: string;
	path: string;
	routeId: string;
	build: string;
	width: number;
	browser: string;
	role: string;
	practice: string;
	time: string;
}

export function readContext(
	shell: Shell,
	screen: HostScreen,
	routeId: string
): Context {
	return {
		screenTitle: screen.title,
		path: screen.path,
		routeId,
		build: '982df75',
		width: globalThis.innerWidth ?? 0,
		browser: browserName(),
		role: copy[shell].role,
		practice: PRACTICE_NAME,
		time: new Date().toLocaleString('en-US', {
			dateStyle: 'medium',
			timeStyle: 'short',
		}),
	};
}

function browserName(): string {
	const agent = globalThis.navigator?.userAgent ?? '';
	if (agent.includes('Firefox/')) return 'Firefox';
	if (agent.includes('Edg/')) return 'Edge';
	if (agent.includes('Chrome/')) return 'Chrome';
	if (agent.includes('Safari/')) return 'Safari';
	return 'Unknown browser';
}
