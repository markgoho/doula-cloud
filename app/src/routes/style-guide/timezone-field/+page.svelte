<script lang="ts">
	import TimezoneField from '#lib/components/molecules/TimezoneField.svelte';
	import { TIMEZONE_HINT, TIMEZONE_NEEDED } from '#lib/timezones.js';

	/*
	 * The longest realistic value, not a representative one (ADR-0025).
	 * The seven US options are labeled, and the longest of those labels is
	 * Phoenix's; but the option list also grows an entry for a zone the
	 * Practice already holds that the seven do not carry, and that entry
	 * is the raw IANA name. "America/Indiana/Indianapolis" is the longest
	 * such name a US Practice can hold, so it is what is selected here.
	 */
	let value = $state('America/Indiana/Indianapolis');
</script>

<stack-l space="var(--space-6)">
	<h1>Timezone field</h1>

	<section>
		<h2>Default</h2>
		<TimezoneField bind:value hint={TIMEZONE_HINT} />
	</section>

	<section>
		<h2>With an error</h2>
		<TimezoneField value="" hint={TIMEZONE_HINT} error={TIMEZONE_NEEDED} />
	</section>
</stack-l>
