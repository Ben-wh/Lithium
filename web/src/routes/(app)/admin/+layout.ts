import { redirect } from '@sveltejs/kit';
import type { LayoutLoad } from './$types';

// Hides the admin area from non-admins. This is UI-only: every admin API
// endpoint must also check the role on the server.
export const load: LayoutLoad = async ({ parent }) => {
	const { user } = await parent();
	if (user?.role !== 'admin') redirect(307, '/');
};
