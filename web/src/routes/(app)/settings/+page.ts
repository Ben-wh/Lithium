import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';

// Admins only. The page hides itself from everyone else; every settings API
// endpoint checks the role on the server too.
export const load: PageLoad = async ({ parent }) => {
	const { user } = await parent();
	if (user?.role !== 'admin') error(404, 'Page not found');
};
