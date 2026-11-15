export interface ColumnOrder {
	order: string;
	column: string;
	direction: 'ascending' | 'descending';
}

/*
 * The order a press on a column's heading asks for (#2955): the column's
 * first listed order when the rows are not in order by it, the opposite
 * direction when they are, and nothing when the column has no order to
 * move to, so its heading is plain text.
 */
export function headingOrder(
	orders: readonly ColumnOrder[],
	ordered: { column: string; direction: 'ascending' | 'descending' } | undefined,
	column: string
): string | undefined {
	const own = orders.filter((entry) => entry.column === column);
	if (ordered?.column !== column) return own[0]?.order;
	return own.find((entry) => entry.direction !== ordered.direction)?.order;
}
