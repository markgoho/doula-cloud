/**
Whether a page's title is still waiting on its own read (#2246).

SvelteKit's route announcer reads `document.title` once, as the new page
mounts. A page titled from what it loads is, at that instant, a
placeholder or nothing, and a screen reader heard "— DoulaCloud". So the
app announces a move itself (`RouteAnnouncer`), once no `PageTitle` is
held here. Every Template with a `loading` state holds its title for as
long as that state lasts, so no route has to say anything of its own.

A count, not a flag: on a move the old page's title lets go and the new
page's takes hold in the same flush, in an order nobody promises.
*/
import { untrack } from 'svelte';

let held = $state(0);

/**
Holds the announcement until the returned function is called.
*/
export function holdAnnouncement(): () => void {
	// Untracked: a `PageTitle` holds from inside its own effect, which must
	// not come to depend on the count it is changing.
	untrack(() => (held += 1));
	return release;
}

function release(): void {
	held -= 1;
}

export function isAnnouncementHeld(): boolean {
	return held > 0;
}
