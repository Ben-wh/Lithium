// Workspace backgrounds. Keep in sync with the server's Backgrounds list and
// the .app-bg-* classes in app.css.
export const backgrounds = [
	{ value: 'none', label: 'None' },
	{ value: 'dots', label: 'Dots' },
	{ value: 'grid', label: 'Grid' },
	{ value: 'lines', label: 'Diagonal lines' },
	{ value: 'waves', label: 'Waves' },
	{ value: 'glow', label: 'Soft glow' }
] as const;

export type Background = (typeof backgrounds)[number]['value'];
