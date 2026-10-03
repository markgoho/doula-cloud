/*
 * Shared by the primitives' render specs (`stack.spec.ts`,
 * `cover.spec.ts`, `cluster.spec.ts`): a component's own root-element reset, written in the
 * layer a component's scoped `<style>` compiles into, and the cleanup for
 * everything a test mounts. `mountForTest` registers an element for
 * removal; `removeMounted` runs in each spec's `afterEach`.
 */

const mounted: Element[] = [];

export function mountForTest<T extends Element>(element: T): T {
	mounted.push(element);
	return element;
}

export function removeMounted(): void {
	for (const element of mounted) element.remove();
	mounted.length = 0;
}

export function resetMarginInComponentsLayer(selector: string): void {
	const style = document.createElement('style');
	style.textContent = `@layer components {\n\t${selector} { margin: 0; }\n}`;
	document.head.append(mountForTest(style));
}
