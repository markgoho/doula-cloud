/*
 * PROTOTYPE (#1926) -- the copy and data every screen in the type-scale
 * prototype renders. A Practice's real content, at the length a real
 * Practice has it, so each candidate scale is read on what it will hold.
 */
import type { JourneyStep } from '#lib/components/organisms/StepRail.svelte';
import type { NavItem } from '#lib/components/organisms/StaffTopBar.svelte';
import type { PracticeOption } from '#lib/components/molecules/PracticeSwitcher.svelte';

export type ScreenKey = 'clients' | 'question' | 'record' | 'overview';

export const PRACTICE_NAME = 'Highland Midwifery & Birth Support Collective';

/**
Every placeholder link stays on this route, because the artifact has nowhere else to go.
*/
export const stay = (name: string) => `#/prototype-type-scale?nav=${encodeURIComponent(name)}`;

export const staffNav: NavItem[] = ['Overview', 'Clients', 'Schedule', 'On call', 'Invoices', 'Settings'].map(
	(label) => ({ label, href: stay(label), current: label === 'Clients' })
);

export const practices: PracticeOption[] = [
	{ practiceId: 'p_7f3a', practiceName: PRACTICE_NAME, roles: ['owner'], href: stay('practice') }
];

export interface ClientRow {
	id: string;
	name: string;
	email: string;
	engagement: string;
	due: string;
	invite: string;
	updated: string;
}

export const clients: ClientRow[] = [
	{
		id: 'c1',
		name: 'Anne-Marie Ochieng-Whitfield',
		email: 'anne-marie.ochieng-whitfield@example.test',
		engagement: 'Birth · Contract signed · Outstanding ($4,250.00)',
		due: '2 April 2027',
		invite: 'Accepted',
		updated: '1 Mar 2027, 9:14am'
	},
	{
		id: 'c2',
		name: 'Persephone Adeyemi-Wollstonecraft',
		email: 'persephone@example.test',
		engagement: 'Postpartum · Contract sent',
		due: '18 May 2027',
		invite: 'Sent, not opened',
		updated: '28 Feb 2027, 4:02pm'
	},
	{
		id: 'c3',
		name: 'Renata Okonkwo',
		email: 'renata.okonkwo@example.test',
		engagement: 'Birth · No contract yet',
		due: '9 June 2027',
		invite: 'Never invited',
		updated: '26 Feb 2027, 11:30am'
	},
	{
		id: 'c4',
		name: 'Maya Lindqvist',
		email: 'maya.l@example.test',
		engagement: 'Birth · Contract signed · Paid',
		due: '14 March 2027',
		invite: 'Accepted',
		updated: '25 Feb 2027, 8:47am'
	},
	{
		id: 'c5',
		name: 'Jo Tran',
		email: 'jo.tran@example.test',
		engagement: 'Postpartum · Contract draft',
		due: '30 July 2027',
		invite: 'Expired, invite again',
		updated: '20 Feb 2027, 6:15pm'
	},
	{
		id: 'c6',
		name: 'Fatima Al-Sayed',
		email: 'fatima.alsayed@example.test',
		engagement: 'Birth · Contract signed · Outstanding ($1,200.00)',
		due: '22 April 2027',
		invite: 'Accepted',
		updated: '18 Feb 2027, 2:09pm'
	}
];

export const journeySteps: JourneyStep[] = [
	{ label: 'Who the Client is', href: stay('name'), status: 'current' },
	{ label: 'How to reach the Client', href: stay('reach'), status: 'todo' },
	{ label: 'Where the Client lives', href: stay('lives'), status: 'todo' },
	{ label: 'Check your answers', href: stay('check'), status: 'todo' }
];

export const visits = [
	{ date: '18 February 2027', doula: 'Persephone Adeyemi-Wollstonecraft', kind: 'Prenatal visit at home' },
	{ date: '4 March 2027', doula: 'Renata Okonkwo', kind: 'Prenatal visit at home' },
	{ date: '21 March 2027', doula: 'Unassigned', kind: 'Postpartum visit at home' }
];

export const offers = [
	{ client: 'Anne-Marie Ochieng-Whitfield', received: '2 March 2027', status: 'Awaiting reply' },
	{ client: 'Persephone Adeyemi-Wollstonecraft', received: '28 February 2027', status: 'Awaiting reply' },
	{ client: 'Renata Okonkwo', received: '26 February 2027', status: 'Accepted' }
];
