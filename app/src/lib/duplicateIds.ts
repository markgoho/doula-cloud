/**
Every id that appears more than once under `root`, in the order each one
is caught repeating itself -- that is, by second occurrence, not first --
each named once however many copies of it there are.

An id has to be unique in a document -- `getElementById`, an `<label for>`
and every `aria-*` reference resolve to the FIRST match in tree order, so a
second copy is not a near-miss, it is a reference pointing at the wrong
element. DataTable is the component that makes this easy to get wrong
(#666): it keeps a `<table>` and a record view as two DOM trees over the
same rows and renders the caller's snippets into both, so an id a caller
assigns exists twice whether or not the copy on screen is the one an
`aria-describedby` resolves to.

Returned rather than asserted so the assertion reads
`expect(findDuplicateIds(container)).toEqual([])` and a failure prints the
offending ids instead of a bare `false`. A hidden element counts: axe's own
duplicate-id rules skip anything under a `display: none` ancestor, which is
exactly why #666 survived the accessibility sweep, so this deliberately
does not ask whether an element is visible.
*/
export function findDuplicateIds(root: ParentNode): string[] {
	const seen = new Set<string>();
	const duplicates = new Set<string>();

	for (const element of root.querySelectorAll('[id]')) {
		const { id } = element;
		if (seen.has(id)) duplicates.add(id);
		seen.add(id);
	}

	return [...duplicates];
}
