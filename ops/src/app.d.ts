// See https://svelte.dev/docs/kit/types#app.d.ts
// for information about these interfaces
declare global {
	namespace App {
		// interface Error {}
		// interface Locals {}
		// interface PageData {}
		// interface PageState {}
		// interface Platform {}
	}
}

// app/'s TextInput, shared through $ui, passes Safari's `passwordrules` (app/src/app.d.ts).
declare module 'svelte/elements' {
	export interface HTMLInputAttributes {
		passwordrules?: string | undefined | null;
	}
}

export {};
