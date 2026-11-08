/*
 * The Name typed into the Clients list, held for the Client hub's Back link.
 * A Client's name is never in a URL (#1829), so the list keeps it in its
 * history entry's state, which the browser's own Back button restores. The
 * hub's "Clients" link is a new navigation, which would drop it, so the list
 * leaves the Name here and the hub hands it back through `goto`'s state.
 * Memory only: a reload forgets it, as it forgets the entry's state.
 */
const held = { name: '' };

export function holdClientsName(name: string): void {
	held.name = name;
}

export function heldClientsName(): string {
	return held.name;
}
