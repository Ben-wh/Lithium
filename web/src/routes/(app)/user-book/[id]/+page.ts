import { error } from '@sveltejs/kit';
import { ApiError, people, tickets } from '$lib/api';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, url, fetch, parent }) => {
	const { user } = await parent();
	if (user?.role !== 'admin' && user?.role !== 'agent') error(404, 'Page not found');
	const view = url.searchParams.get('view') === 'all' ? 'all' : 'active';
	try {
		const [person, history] = await Promise.all([
			people.get(params.id, fetch),
			tickets.list(
				{ requester: Number(params.id), status: view === 'all' ? '' : 'unassigned,in_progress,pending', limit: 50 },
				fetch
			)
		]);
		return { person, history, view };
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) error(404, 'Person not found');
		throw err;
	}
};
