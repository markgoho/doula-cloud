import { afterNavigate } from '$app/navigation';

interface ResultFocus {
	seed: () => void;
	wantedKey: () => string;
	isLanded: () => boolean;
	isOwnChange: () => boolean;
	focus: () => void;
}

export function focusResultAfterChange({ seed, wantedKey, isLanded, isOwnChange, focus }: ResultFocus) {
	let isFocusPending = $state(false);
	let lastKey: string | undefined;
	afterNavigate(() => {
		seed();
		if (lastKey !== undefined && lastKey !== wantedKey() && !isOwnChange()) isFocusPending = true;
		lastKey = wantedKey();
	});
	$effect(() => {
		if (!isFocusPending || !isLanded()) return;
		isFocusPending = false;
		focus();
	});
}
