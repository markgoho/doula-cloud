/**
 * What a save says on the screen it lands on (#1710).
 *
 * A save that stays where it was made says what it did with a `Notice` in
 * place. A save that leaves the screen -- the details journey's check
 * page, the Client edit form, intake, the Start work form -- has nothing
 * in place to say it with, so it hands the message to the screen it goes
 * to, and that screen shows it before its `<h1>`, focused on arrival
 * (`docs/design/govuk-alignment.md`, Confirmation pages and Notification
 * banner).
 *
 * ## Why module memory, and not the URL or the history entry
 *
 * The message is shown once: a reload or a later visit does not show it
 * again. A query such as `?started=true` (#1611's first carrier) is
 * still in the address bar after a reload, so the page says it again.
 * SvelteKit's `page.state` is restored when the reader comes Back to the
 * entry, so the page says it again then. This module holds the message
 * only while the navigation runs: the destination reads it while it
 * renders, which is before `goto` settles, and it is gone once `goto`
 * settles whether the destination read it or not. A reload starts with an
 * empty module, so nothing is shown.
 */
import { goto } from '$app/navigation';

interface Pending {
	pathname: string;
	message: string;
}

const carried: { pending?: Pending } = {};

// An href in this app is an absolute path; the origin is only there to
// parse it.
function pathnameOf(href: string): string {
	return new URL(href, 'http://localhost').pathname;
}

/**
 * Navigates to `href`, and gives `message` to the screen there.
 */
export async function gotoWithOutcome(href: string, message: string): Promise<void> {
	carried.pending = { pathname: pathnameOf(href), message };
	try {
		await goto(href);
	} finally {
		carried.pending = undefined;
	}
}

/**
 * The message a save sent to this screen, read once while the screen
 * renders. Undefined on any visit no save sent, and on a screen other
 * than the one the save went to.
 */
export function takeOutcome(url: Pick<URL, 'pathname'>): string | undefined {
	const { pending } = carried;
	if (pending?.pathname !== url.pathname) return undefined;
	const { message } = pending;
	carried.pending = undefined;
	return message;
}
