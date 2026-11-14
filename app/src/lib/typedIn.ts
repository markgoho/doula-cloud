export function typedIn(field: HTMLElement | undefined, held: string): string {
	return (field?.querySelector('input')?.value ?? held).trim();
}
