import { getSettings } from '$lib/api';
import type { LayoutLoad } from './$types';

// Workspace settings shape what every page shows (e.g. the ticket ID prefix).
// Re-run with invalidate('app:settings') after an admin changes them.
export const load: LayoutLoad = async ({ fetch, depends }) => {
	depends('app:settings');
	return { settings: await getSettings(fetch) };
};
