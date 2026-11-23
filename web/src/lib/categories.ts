// Category colours. Keep in sync with the server's validColors and the
// .tag-* classes in app.css.
export const categoryColors = [
	{ value: 'gray', label: 'Gray' },
	{ value: 'red', label: 'Red' },
	{ value: 'orange', label: 'Orange' },
	{ value: 'amber', label: 'Amber' },
	{ value: 'green', label: 'Green' },
	{ value: 'teal', label: 'Teal' },
	{ value: 'blue', label: 'Blue' },
	{ value: 'indigo', label: 'Indigo' },
	{ value: 'purple', label: 'Purple' },
	{ value: 'pink', label: 'Pink' }
] as const;

export type CategoryColor = (typeof categoryColors)[number]['value'];

/** Suggest a colour nobody's using yet, so new categories are easy to tell apart. */
export function nextColor(used: string[]): CategoryColor {
	const free = categoryColors.find((c) => c.value !== 'gray' && !used.includes(c.value));
	return free?.value ?? 'gray';
}
