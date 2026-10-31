import type { Attachment } from 'svelte/attachments';

/*
 * #2235, #2335: a panel in the shell that holds links -- the narrow Staff
 * sheet, a header menu's popover -- shuts when one of its links is pressed.
 * A full page load would close it, but SvelteKit's client-side navigation
 * keeps the shell mounted, so the panel stayed open over the page the link
 * chose.
 *
 * It shuts as the link is pressed, on the panel's own listener, before the
 * press bubbles up to the router's listener on the document. Shutting hands
 * focus back to whatever opened the panel, and the router then resets focus
 * for the new page, as after any other link. Shut once the new page is in,
 * the panel would take that focus back to its opener.
 *
 * Enter on a link fires the same click, so a keyboard press is covered. A
 * press anywhere else in the panel, a button included, is left alone.
 */
export function closeOnLinkPress<T extends HTMLElement>(close: (panel: T) => void): Attachment<T> {
	return (panel) => {
		function handleClick(event: MouseEvent) {
			if (event.target instanceof Element && event.target.closest('a[href]')) {
				close(panel);
			}
		}
		panel.addEventListener('click', handleClick);
		return () => panel.removeEventListener('click', handleClick);
	};
}
