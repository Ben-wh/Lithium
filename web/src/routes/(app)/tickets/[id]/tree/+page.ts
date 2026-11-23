import { error } from '@sveltejs/kit';
import { ApiError, tickets } from '$lib/api';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, fetch, parent }) => {
	const { user, settings } = await parent();
	if (user?.role !== 'admin' && user?.role !== 'agent') error(404, 'Ticket not found');
	if (!settings.incidentTree) error(404, 'Incident trees are turned off');
	try {
		return { tree: await tickets.tree(params.id, fetch) };
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) {
			error(404, err.message.charAt(0).toUpperCase() + err.message.slice(1));
		}
		throw err;
	}
};
