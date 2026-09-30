/**
 * The teaser's waitlist form, whose list Buttondown keeps (ADR-0014).
 *
 * The form posts to Buttondown's embed URL, which #366 records once the
 * account exists. Until then this is `undefined`, and the teaser shows no
 * form: a form with no address would take a person's name and lose it.
 * Setting it is the whole change that turns the form on, together with a
 * new version of the Privacy Policy that names Buttondown (ADR-0054).
 */
export const WAITLIST_FORM_ACTION: string | undefined = undefined;

/**
 * The four campaign parameters the form carries as hidden inputs, each
 * posted as `metadata__<name>` so that Buttondown keeps it on the
 * subscriber (ADR-0016, amended on #368). They are a one-way door: a
 * person who joins before they exist can never be tagged afterwards.
 */
export const UTM_PARAMETERS = ['utm_source', 'utm_medium', 'utm_campaign', 'utm_content'] as const;

/**
 * Copies each campaign parameter in `search` into its hidden input.
 *
 * It runs in the page as an inline script (`utmScript` below), not
 * through SvelteKit, which ships no JavaScript on this site (`csr =
 * false`). So it must be self-contained: it names the four parameters
 * itself and reaches for nothing outside its own body, because
 * `utmScript` is its source text. If it is blocked, the hidden inputs
 * post empty and the signup still works.
 */
export function fillUtmFields(search: string): void {
	const parameters = new URLSearchParams(search);
	for (const name of ['utm_source', 'utm_medium', 'utm_campaign', 'utm_content']) {
		const value = parameters.get(name);
		const input = document.querySelector(`input[name="${CSS.escape(`metadata__${name}`)}"]`);
		if (value && input instanceof HTMLInputElement) input.value = value;
	}
}

// The inline script the form carries after its last input.
export const utmScript = `(${fillUtmFields.toString()})(location.search);`;

// The same script as the element the page inlines. Built here and not in
// the component, where a closing script tag would end the component's own
// script block.
export const utmScriptElement = `<script>${utmScript}</script>`;
