/**
 * Shared machinery for the Every Layout-inspired custom elements (issue #98,
 * ADR-0003). Each primitive's *default* state is styled by a plain-CSS rule
 * already present in the document stylesheet (`styles/primitives.css`) --
 * zero JS required. This module only runs JS to override that default when
 * an instance sets a non-default attribute value: it computes a resolved
 * config, stamps it as `data-i` on the element, and injects a scoped
 * `<style>` (deduplicated by resolved config, so identical configs share one
 * stylesheet) into `document.head`, layered into `@layer utilities` --
 * `primitives.css`'s defaults live in the same layer (ADR-0003 names it the
 * page-layout utility layer) -- so it beats the tag-name default on
 * specificity without needing `!important`.
 */

export type PropertyDefaults = Record<string, string>;

function resolveValues<Defaults extends PropertyDefaults>(
	element: HTMLElement,
	defaults: Defaults
): Defaults {
	const resolved = { ...defaults };
	for (const key of Object.keys(defaults)) {
		const attributeValue = element.getAttribute(key);
		if (attributeValue) {
			resolved[key as keyof Defaults] = attributeValue as Defaults[keyof Defaults];
		}
	}
	return resolved;
}

function serialize(values: PropertyDefaults): string {
	return Object.entries(values)
		.map(([key, value]) => `${key}:${value}`)
		.join(';');
}

const injectedStyleIds = new Set<string>();

function injectStyle(tagName: string, id: string, cssText: string): void {
	const styleId = `${tagName}__${id}`;
	if (injectedStyleIds.has(styleId)) return;
	injectedStyleIds.add(styleId);

	const style = document.createElement('style');
	style.dataset.layoutPrimitiveStyle = styleId;
	style.textContent = `@layer utilities {\n${cssText}\n}`;
	document.head.append(style);
}

/**
 * Builds (without registering) a custom element class whose non-default
 * styling is computed by `css(resolvedValues, selector)`, where `selector`
 * is this instance's `tagName[data-i="..."]` attribute selector -- `css`
 * returns full rule text (host rule, child-combinator rules, whatever the
 * primitive needs), not just a declaration block, since primitives like
 * Stack target children rather than the host element itself. `defaults`
 * lists only the *string*-valued props (arbitrary CSS values); boolean
 * toggles are handled by plain `[attr]` CSS selectors in `primitives.css`
 * and never reach this module.
 *
 * Exported as a class (not auto-registered) so a primitive that needs more
 * than CSS injection -- Icon's `label`, which reflects to `role`/
 * `aria-label` -- can subclass it and add that behavior alongside the CSS
 * sync. Primitives with no such extra behavior should use
 * `defineLayoutPrimitive` instead, which registers directly.
 */
export function createLayoutPrimitiveClass<Defaults extends PropertyDefaults>(
	tagName: string,
	defaults: Defaults,
	css: (values: Defaults, selector: string) => string
) {
	const attributeNames = Object.keys(defaults);

	return class LayoutPrimitiveElement extends HTMLElement {
		static get observedAttributes(): string[] {
			return attributeNames;
		}

		#sync(): void {
			const values = resolveValues(this, defaults);
			const isDefault = attributeNames.every((key) => values[key] === defaults[key]);

			if (isDefault) {
				delete this.dataset.i;
				return;
			}

			const id = serialize(values);
			this.dataset.i = id;
			injectStyle(tagName, id, css(values, `${tagName}[data-i="${id}"]`));
		}

		connectedCallback(): void {
			this.#sync();
		}

		// Params unused here, but the wider signature (vs. plain `(): void`) lets
		// subclasses like Icon override with the real customElements signature.
		// eslint-disable-next-line @typescript-eslint/no-unused-vars
		attributeChangedCallback(name?: string, oldValue?: string | null, newValue?: string | null): void {
			this.#sync();
		}
	};
}

/**
Builds and registers a primitive class in one step -- see {@link createLayoutPrimitiveClass}.
*/
export function defineLayoutPrimitive<Defaults extends PropertyDefaults>(
	tagName: string,
	defaults: Defaults,
	css: (values: Defaults, selector: string) => string
): void {
	customElements.define(tagName, createLayoutPrimitiveClass(tagName, defaults, css));
}
