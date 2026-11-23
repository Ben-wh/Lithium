// Light / dark / system theme preference. The initial theme is applied by an
// inline script in app.html before first paint; keep the two in sync.

export type ThemePreference = 'light' | 'dark' | 'system';

const STORAGE_KEY = 'lithium-theme';
const media = matchMedia('(prefers-color-scheme: light)');

function readPreference(): ThemePreference {
	try {
		const v = localStorage.getItem(STORAGE_KEY);
		if (v === 'light' || v === 'dark') return v;
	} catch {
		/* storage blocked */
	}
	return 'system';
}

function apply(pref: ThemePreference) {
	const light = pref === 'light' || (pref === 'system' && media.matches);
	document.documentElement.dataset.theme = light ? 'light' : 'dark';
	document
		.querySelector('meta[name="theme-color"]')
		?.setAttribute('content', light ? '#f6f6f8' : '#0d0d10');
}

class Theme {
	preference = $state<ThemePreference>(readPreference());

	constructor() {
		// Follow the OS when it changes, if the user hasn't picked a side.
		media.addEventListener('change', () => {
			if (this.preference === 'system') apply('system');
		});
	}

	set(pref: ThemePreference) {
		this.preference = pref;
		try {
			if (pref === 'system') localStorage.removeItem(STORAGE_KEY);
			else localStorage.setItem(STORAGE_KEY, pref);
		} catch {
			/* storage blocked; still applies for this page view */
		}
		apply(pref);
	}
}

export const theme = new Theme();
