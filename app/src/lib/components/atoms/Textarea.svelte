<!--
	The long-text control, and the two decisions it carries.

	## Growth: bounded, and CSS-only

	`rows` is a starting height, never a cap. Growth is `field-sizing:
	content` -- one declaration, no ResizeObserver, no mirrored shadow node
	-- so a contract body stops being a five-line porthole onto a thousand
	words. It is Baseline newly available (Chrome 123, Safari 26.2, Firefox
	152); where it is unsupported the `rows` attribute still applies and the
	field is simply fixed, which is what it is today.

	Growth is bounded at both ends, which is what keeps it inside ADR-0020:
	`min-block-size` holds the starting height so an empty field cannot
	collapse, and `max-block-size` stops the field pushing the submit button
	off the screen -- past it the textarea scrolls, as a textarea always
	has. Movement is only ever a direct response to a keystroke, and never
	unbounded.

	## The character count: only against a real limit

	`maxLength` is optional and almost nothing passes it. GOV.UK's rule is
	kept verbatim -- a counter on a field with no limit reads as a target
	and makes people write to it -- and #468 established that exactly one
	field in this application has a server-enforced maximum: the two
	Practice website facts, at `website.MaxFactLength`, refused by the
	handler and again by 00045's CHECK constraint.

	Where it does ship, the shape is govuk-frontend's, with the parts this
	app does not need left out. There are three nodes, not one:

	- a static description, visually hidden, naming the budget. It is what
	  `aria-describedby` points at, so the limit is announced on arrival
	  rather than only after typing.
	- a visually hidden `aria-live="polite"` region carrying the running
	  count, updated one second after the last keystroke. A live region on
	  the visible node would announce every character typed.
	- the visible count, `aria-hidden`, so the same number is not spoken
	  twice.

	No `maxlength` attribute: a hard cap silently truncates pasted text.
	Going over is a state the person can see and fix -- the count goes
	negative and turns red -- and the refusal belongs to the caller's
	submit and to the handler behind it.

	Characters are counted as code points, not UTF-16 code units, because
	`api/internal/website` counts them with `utf8.RuneCountInString`. An
	emoji costs one there, so it costs one here; `.length` would show a
	person a budget the server does not enforce.
-->
<script lang="ts">
	/*
	 * govuk-frontend's own figure: long enough that a touch typist is not
	 * interrupted mid-word, short enough that the count is current by the
	 * time anyone reaches for it.
	 */
	const ANNOUNCE_AFTER_MS = 1000;

	const generatedId = $props.id();

	let {
		id = generatedId,
		value,
		onInput,
		name,
		placeholder,
		required = false,
		disabled = false,
		invalid = false,
		describedBy,
		/*
		 * For the two field-template editors, whose option lists sit in a
		 * repeating row with no label of their own. Every other caller
		 * wraps this in `LabeledField` and leaves it unset.
		 */
		ariaLabel,
		rows = 4,
		maxLength
	}: {
		id?: string;
		value: string;
		onInput: (value: string) => void;
		name?: string;
		placeholder?: string;
		required?: boolean;
		disabled?: boolean;
		invalid?: boolean;
		describedBy?: string;
		ariaLabel?: string;
		rows?: number;
		maxLength?: number;
	} = $props();

	const isCounted = $derived(maxLength !== undefined);
	const budgetId = $derived(`${id}-budget`);
	const remaining = $derived((maxLength ?? 0) - [...value].length);
	const isOverLimit = $derived(isCounted && remaining < 0);

	/*
	 * The budget, not the running count. The count is a live region, and a
	 * live region named by aria-describedby is read twice.
	 */
	const control = $derived(
		[describedBy, isCounted ? budgetId : undefined].filter(Boolean).join(' ') || undefined
	);

	function pluralize(count: number, noun: string): string {
		return `${count} ${noun}${count === 1 ? '' : 's'}`;
	}

	const countMessage = $derived(
		remaining < 0
			? `You have ${pluralize(-remaining, 'character')} too many`
			: `You have ${pluralize(remaining, 'character')} remaining`
	);

	let announcedMessage = $state('');

	$effect(() => {
		const pending = countMessage;
		const timer = setTimeout(() => (announcedMessage = pending), ANNOUNCE_AFTER_MS);
		return () => clearTimeout(timer);
	});
</script>

<textarea
	{id}
	{name}
	{value}
	{rows}
	{placeholder}
	{required}
	{disabled}
	aria-label={ariaLabel}
	class:invalid
	style:--textarea-rows={rows}
	aria-invalid={invalid}
	aria-describedby={control}
	oninput={(event_) => onInput(event_.currentTarget.value)}
></textarea>

{#if maxLength !== undefined}
	<p id={budgetId} class="visually-hidden">
		You can enter up to {pluralize(maxLength, 'character')}
	</p>
	<p class="visually-hidden" aria-live="polite">{announcedMessage}</p>
	<p class="count" class:over={isOverLimit} aria-hidden="true">{countMessage}</p>
{/if}

<style>
	@layer components {
		textarea {
			/*
			 * The ceiling, in lines. Deliberately generous -- it is there to
			 * stop a thousand-word contract body from becoming the whole
			 * page, not to discourage a long answer -- and unitless, so it
			 * scales with whatever line height the type scale gives the
			 * control.
			 */
			--textarea-max-lines: 20;

			inline-size: 100%;
			padding: var(--space-2) var(--space-3);
			color: var(--color-on-surface);
			background-color: var(--color-surface);
			border: var(--border-thin) solid var(--color-outline);
			border-radius: var(--radius);
			/*
			 * Where `field-sizing` is unsupported this is the whole story
			 * and `rows` sets the height; where it is supported these two
			 * become the floor and the ceiling of the growth.
			 */
			min-block-size: calc(var(--textarea-rows) * 1lh);
			max-block-size: calc(var(--textarea-max-lines) * 1lh);
		}

		@supports (field-sizing: content) {
			textarea {
				field-sizing: content;
			}
		}

		textarea.invalid {
			border-color: var(--color-error);
		}

		textarea:disabled {
			cursor: not-allowed;
			opacity: 0.6;
		}

		textarea:focus-visible {
			outline: var(--focus-ring-width) solid var(--color-primary);
			outline-offset: var(--focus-ring-offset);
		}

		.count {
			margin: 0;
			color: var(--color-on-surface-muted);
			font-size: var(--text-body-sm-size);
		}

		.count.over {
			color: var(--color-error);
			font-weight: var(--font-weight-medium);
		}

		/* WCAG-standard clip technique: stays in the accessibility tree and
		   readable by AT/voice-control/translation tools, unlike aria-label
		   which strips real DOM text out of those paths. */
		/* tokens:ignore -- the WCAG clip technique's own geometry, not a
		   design value. The 1px box and the -1px pull are what the
		   technique is; a token would imply somebody may retune them. */
		.visually-hidden {
			position: absolute;
			inline-size: 1px;
			block-size: 1px;
			margin: -1px;
			padding: 0;
			overflow: hidden;
			clip: rect(0, 0, 0, 0);
			white-space: nowrap;
			border: 0;
		}
	}
</style>
