import { registerDataPrimitives } from './primitives.js';
import { defineIcon } from './icon.js';

/** Registers all 13 Every Layout-inspired custom elements. Browser-only -- call from a client entry point (e.g. `onMount`), never during SSR. */
export function registerLayoutPrimitives(): void {
	registerDataPrimitives();
	defineIcon();
}
