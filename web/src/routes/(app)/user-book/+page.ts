import { error } from '@sveltejs/kit';
import { people } from '$lib/api';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ url, fetch, parent }) => {
	const { user } = await parent();
	if (user?.role !== 'admin' && user?.role !== 'agent') error(404, 'Page not found');
	const q = url.searchParams.get('q')?.trim() ?? '';
	return { q, ...(await people.list(q, 0, fetch)) };
};
