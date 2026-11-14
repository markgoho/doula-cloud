export function rowLinkLabeler<T>(
	rows: readonly T[],
	name: (row: T) => string,
	detail: (row: T) => string
): (row: T) => string {
	const counts = new Map<string, number>();
	for (const row of rows) counts.set(name(row), (counts.get(name(row)) ?? 0) + 1);
	return (row) => (counts.get(name(row))! > 1 ? `${name(row)}, ${detail(row)}` : name(row));
}
