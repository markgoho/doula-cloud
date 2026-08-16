import { createLayoutPrimitiveClass } from './defineLayoutPrimitive.js';

/**
 * Icon (`<icon-l>`) -- the one primitive with a prop that isn't a CSS value.
 * `space` reuses the shared CSS-injection machinery; `label` reflects to
 * `role="img"`/`aria-label` instead, which the shared machinery can't
 * express, so this subclasses it to add that behavior alongside the CSS
 * sync. See ADR-0003 and issue #98.
 */
// Building the class (which `extends HTMLElement`) must stay inside this
// function, not at module top level -- `+layout.svelte` imports this module
// during SSR, before `HTMLElement` exists in Node, and only *calls*
// `defineIcon` client-side (onMount).
export function defineIcon(): void {
	const IconBase = createLayoutPrimitiveClass(
		'icon-l',
		{ space: '' },
		(v, s) => `${s} { margin-inline-end: ${v.space}; }`
	);

	class IconElement extends IconBase {
		static get observedAttributes(): string[] {
			return [...super.observedAttributes, 'label'];
		}

		#syncLabel(): void {
			const label = this.getAttribute('label');
			if (label) {
				this.setAttribute('role', 'img');
				this.setAttribute('aria-label', label);
			} else {
				this.removeAttribute('role');
				this.removeAttribute('aria-label');
			}
		}

		connectedCallback(): void {
			super.connectedCallback();
			this.#syncLabel();
		}

		attributeChangedCallback(name: string, oldValue: string | null, newValue: string | null): void {
			super.attributeChangedCallback(name, oldValue, newValue);
			if (name === 'label') this.#syncLabel();
		}
	}

	customElements.define('icon-l', IconElement);
}
