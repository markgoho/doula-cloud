import { expect, it, vi } from 'vitest';
import { saveFile } from './invoiceExport.js';

it('clicks a download link for the file and cleans up', () => {
	const create = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:x');
	const revoke = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {});
	const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
		expect(this.download).toBe('a.csv');
		expect(this.href).toBe('blob:x');
	});

	saveFile(new Blob(['x']), 'a.csv');

	expect(click).toHaveBeenCalledOnce();
	expect(revoke).toHaveBeenCalledWith('blob:x');
	expect(document.querySelector('a[download]')).toBeNull();
	create.mockRestore();
	revoke.mockRestore();
	click.mockRestore();
});
