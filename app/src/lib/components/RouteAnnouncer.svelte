<script lang="ts">
	/**
	 * The one region that tells a screen reader a move has happened (#2246),
	 * in place of SvelteKit's own (hidden in `base.css`). SvelteKit's reads
	 * `document.title` the instant the new page mounts, before a page titled
	 * from its own read knows its title, so it said a placeholder, or just
	 * "— DoulaCloud". This one reads it once the page's title has settled:
	 * once no Template is still drawing its loading state
	 * (`routeAnnouncement.svelte.ts`). Like `PageTitle`, it draws nothing a
	 * sighted reader sees, so it is not tiered and has no style-guide page.
	 *
	 * The first load is not a move, and is not announced: the browser reads
	 * that page's title itself, as SvelteKit's own announcer assumes.
	 */
	import { afterNavigate } from '$app/navigation';
	import { isAnnouncementHeld } from '#lib/routeAnnouncement.svelte.js';

	let isFirstLoad = true;
	let isAwaiting = $state(false);
	let announced = $state('');

	afterNavigate(() => {
		if (isFirstLoad) {
			isFirstLoad = false;
			return;
		}
		isAwaiting = true;
	});

	$effect(() => {
		if (!isAwaiting || isAnnouncementHeld()) return;
		isAwaiting = false;
		announced = document.title || 'untitled page';
	});
</script>

<div class="visually-hidden" aria-live="assertive" aria-atomic="true">{announced}</div>
