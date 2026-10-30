import { invalidate } from '$app/navigation';
import { PRACTICE_SESSION_DEPENDENCY } from './practiceSession.js';

/**
 * Reads the Practice session again when a Connect status read says
 * otherwise about whether Clients can pay (#2295). The session reads that
 * fact once, and Stripe's webhook changes it outside any act in the app,
 * so a screen that reads Connect status is where the app learns first.
 * Both sides read the same stored column, so one read of the session
 * settles it. Called from a read's own result, never from an effect over
 * the session, so a fresh session cannot start another read.
 */
export function refreshSessionIfClientsCanPayChanged(
	session: { clientsCanPay?: boolean },
	clientsCanPay: boolean | undefined
): void {
	const isKnownOnBothSides = session.clientsCanPay !== undefined && clientsCanPay !== undefined;
	if (isKnownOnBothSides && session.clientsCanPay !== clientsCanPay) {
		void invalidate(PRACTICE_SESSION_DEPENDENCY);
	}
}
