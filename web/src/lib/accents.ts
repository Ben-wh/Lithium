// Workspace accent colours, all pastel. Keep in sync with the server's
// Accents list and the [data-accent] blocks in app.css. `swatch` is the
// main (400) shade, for previews.
export const accents = [
	{ value: 'coral', label: 'Coral', swatch: '#f08f8f' },
	{ value: 'peach', label: 'Peach', swatch: '#f59e6b' },
	{ value: 'butter', label: 'Butter', swatch: '#f2c94c' },
	{ value: 'mint', label: 'Mint', swatch: '#6dd3a3' },
	{ value: 'aqua', label: 'Aqua', swatch: '#6fcfc4' },
	{ value: 'sky', label: 'Sky', swatch: '#7fb2f7' },
	{ value: 'periwinkle', label: 'Periwinkle', swatch: '#9a9ef4' },
	{ value: 'lavender', label: 'Lavender', swatch: '#b48ff0' },
	{ value: 'pink', label: 'Pink', swatch: '#ef91c1' }
] as const;

export type Accent = (typeof accents)[number]['value'];

const STORAGE_KEY = 'lithium-accent';

/**
 * Apply the workspace accent, and remember it so app.html can apply it
 * before first paint on the next visit (no flash of coral).
 */
export function applyAccent(accent: string) {
	const known = accents.some((a) => a.value === accent);
	const value = known && accent !== 'coral' ? accent : '';
	if (value) document.documentElement.dataset.accent = value;
	else delete document.documentElement.dataset.accent;
	try {
		if (value) localStorage.setItem(STORAGE_KEY, value);
		else localStorage.removeItem(STORAGE_KEY);
	} catch {
		/* storage blocked; still applies for this page view */
	}
}
