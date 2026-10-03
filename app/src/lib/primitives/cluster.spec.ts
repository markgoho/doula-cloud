/*
 * The render gate for #1738. `align` is an old HTML presentational
 * attribute, and Chromium maps it to `text-align` on the element that
 * carries it -- a custom element included. So `<cluster-l align="center">`
 * centered the text of every child that inherited from it, not only the
 * children's cross-axis position: a link that wrapped to a second line had
 * that line centered under the first.
 *
 * The attribute's job is `align-items` and nothing else (`primitives.ts`);
 * the browser's own mapping is the other party to that contract, and
 * reading `primitives.css` cannot show it, so it is rendered. These tests read
 * what a child inherits, which is the defect itself, and keep the two
 * things the fix must not break: the alignment the attribute names still
 * applies, and a child still inherits whatever text alignment the
 * cluster's own parent asked for.
 */
import { afterEach, beforeAll, describe, expect, it } from 'vitest';
import { registerLayoutPrimitives } from './index.js';
import { mountForTest, removeMounted } from './layerFixture.js';
import '#lib/styles/app.css';

beforeAll(() => {
	if (!customElements.get('cluster-l')) registerLayoutPrimitives();
});

afterEach(removeMounted);

function mountCluster(
	align: string,
	parentTextAlign: string
): { cluster: HTMLElement; child: HTMLElement } {
	const parent = document.createElement('div');
	parent.style.textAlign = parentTextAlign;

	const cluster = document.createElement('cluster-l');
	cluster.setAttribute('align', align);

	const child = document.createElement('a');
	child.href = '#';
	child.textContent = 'Start new work with Alexandra Featherstonehaugh';

	cluster.append(child);
	parent.append(cluster);
	document.body.append(mountForTest(parent));

	return { cluster, child };
}

describe('cluster-l align', () => {
	// The first five are the values Chromium maps to `text-align`. `baseline`
	// is the one other value a call site uses; the browser does not map it,
	// and it is here so the rule is seen to leave it alone too.
	it.each(['center', 'left', 'right', 'justify', 'middle', 'baseline'])(
		'gives a child no text alignment of its own when align is %s',
		(align) => {
			const { child } = mountCluster(align, 'start');

			expect(getComputedStyle(child).textAlign).toBe('start');
		}
	);

	it('still aligns the children on the cross axis', () => {
		const { cluster } = mountCluster('center', 'start');

		expect(getComputedStyle(cluster).alignItems).toBe('center');
	});

	it("lets a child inherit the text alignment of the cluster's own parent", () => {
		const { child } = mountCluster('center', 'end');

		expect(getComputedStyle(child).textAlign).toBe('end');
	});
});
