import { redirect } from '@sveltejs/kit';
import type { LayoutLoad } from './$types';

// Hides the admin area from non-admins. This is UI-only: every admin API
// endpoint must also check the role on the server.
export const load: LayoutLoad = async ({ parent, url }) => {
	const { user } = await parent();
	if (user?.role !== 'admin') redirect(307, '/');
	// Settings used to be a tab here; keep old links and bookmarks working.
	if (url.searchParams.get('tab') === 'settings') {
		const section = url.searchParams.get('section');
		redirect(307, section ? `/settings?section=${encodeURIComponent(section)}` : '/settings');
	}
};
