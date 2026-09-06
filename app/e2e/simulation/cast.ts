// The per-cast-member record #821's Key Interfaces name: her slug, her
// BrowserContext, her mailbox address, her signed-in state, and the
// simulated instant she last authenticated. Cast is the registry that
// owns their lifecycle for the life of a run -- one persistent context
// per cast member, admitted once, closed once, so two personas' acts
// never share a session and their screenshots never collide (capture.ts
// already keys a screenshot by slug; a distinct context per member is
// what keeps their cookies and signed-in state apart too).
import type { Browser, BrowserContext, Page } from '@playwright/test';
import { readSimulatedNow } from '../stack';
import type { JumpResult } from './clock';

export interface CastMember {
	readonly slug: string;
	readonly context: BrowserContext;
	readonly page: Page;
	readonly mailboxAddress: string;
	signedIn: boolean;
	// The simulated instant (readSimulatedNow()'s ISO string) she last
	// authenticated, or undefined before her first sign-in. Compared
	// against nothing here -- clock.ts's jump() already did the
	// 12h-threshold arithmetic and handed back crossedReauthThreshold, so
	// this field is a record for the run README, not an input to another
	// calculation.
	lastAuthenticatedAt: string | undefined;
	// Set on every open session by markReauthDue(), the caller's response
	// to a jump crossing the reauth threshold (clock.ts's
	// JumpResult.crossedReauthThreshold). Cleared by signInIfDue() once
	// she has actually signed in again -- so "due" survives across
	// however many acts pass before her next turn comes up.
	reauthDue: boolean;
}

// Drives one cast member's real sign-in. What "signing in" means is a
// Staff or a Client-portal flow, which this module has no business
// knowing -- the caller supplies it, the same shape capture.ts's
// narrateWait hook takes. Called with the member so the caller can drive
// her page directly, and can therefore also record it as an ordinary
// observed act (calendar.md counts a jump's re-sign-ins as acts).
export type SignIn = (member: CastMember) => Promise<void>;

// Cast admits one persistent BrowserContext per slug and keeps it for the
// life of a run. It never decides what a member does -- that's the
// scheduler and the persona/extra turn wrappers -- it only owns whether
// her session exists, whether it is signed in, and whether a jump has
// left it needing to sign in again.
export class Cast {
	private readonly members = new Map<string, CastMember>();

	constructor(
		private readonly browser: Browser,
		private readonly signIn: SignIn
	) {}

	// Admits a new cast member with her own BrowserContext and Page. Throws
	// on a repeat slug -- a run never admits the same cast member twice,
	// and a silent second admission would leave one persona's earlier
	// context orphaned rather than closed.
	async admit(slug: string, mailboxAddress: string): Promise<CastMember> {
		if (this.members.has(slug)) {
			throw new Error(`cast: ${slug} is already admitted`);
		}
		const context = await this.browser.newContext();
		const page = await context.newPage();
		const member: CastMember = { slug, context, page, mailboxAddress, signedIn: false, lastAuthenticatedAt: undefined, reauthDue: false };
		this.members.set(slug, member);
		return member;
	}

	get(slug: string): CastMember {
		const member = this.members.get(slug);
		if (!member) {
			throw new Error(`cast: ${slug} was never admitted`);
		}
		return member;
	}

	all(): CastMember[] {
		const members: CastMember[] = [];
		for (const member of this.members.values()) {
			members.push(member);
		}
		return members;
	}

	// The caller's response to clock.ts's JumpResult.crossedReauthThreshold:
	// every session that is currently signed in is marked due. A member who
	// never signed in (an Extra not yet on stage, a Persona not yet past
	// her first sign-in step) has nothing to re-authenticate, so is left
	// alone -- flagging her would just have signInIfDue() sign her in for
	// the first time under a name that means "again".
	markReauthDue(): void {
		for (const member of this.members.values()) {
			if (member.signedIn) {
				member.reauthDue = true;
			}
		}
	}

	// The gate every act must pass through before it runs: if this member
	// is due, sign her in again first. simulatedNow is the caller's
	// stack.ts readSimulatedNow() reading, taken at the moment of sign-in
	// rather than the moment of the jump, since a run resumes across
	// sittings and the two can be calls apart.
	async signInIfDue(slug: string, simulatedNow: string): Promise<void> {
		const member = this.get(slug);
		if (!member.reauthDue) {
			return;
		}
		await this.signIn(member);
		member.signedIn = true;
		member.lastAuthenticatedAt = simulatedNow;
		member.reauthDue = false;
	}

	// A first sign-in is the same call, just never gated on reauthDue --
	// the caller's script decides when a member first signs in (it is
	// itself one of her observed acts), Cast only records that it happened.
	async recordSignedIn(slug: string, simulatedNow: string): Promise<void> {
		const member = this.get(slug);
		member.signedIn = true;
		member.lastAuthenticatedAt = simulatedNow;
		member.reauthDue = false;
	}

	// Wires clock.ts's jump() straight to markReauthDue() -- the caller's
	// job clock.ts's own header comment names ("that is explicitly the
	// caller's job, gated on crossedReauthThreshold"). Below the reauth
	// threshold this is a no-op, exactly as it should be.
	afterJump(result: JumpResult): void {
		if (result.crossedReauthThreshold) {
			this.markReauthDue();
		}
	}

	// The one idiom every turn should be driven through: whatever a member
	// is about to do, this makes sure a jump that left her due gets a
	// sign-in first, so "every open session re-authenticates before its
	// next act" is true by construction rather than by the caller
	// remembering to sequence signInIfDue() before every personaAct()/
	// extraAct() call itself.
	async act<T>(slug: string, work: (member: CastMember) => Promise<T>): Promise<T> {
		await this.signInIfDue(slug, readSimulatedNow());
		return work(this.get(slug));
	}

	async closeAll(): Promise<void> {
		for (const member of this.members.values()) {
			await member.context.close();
		}
		this.members.clear();
	}
}
