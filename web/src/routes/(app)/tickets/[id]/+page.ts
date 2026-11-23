import { error } from '@sveltejs/kit';
import { ApiError, listCategories, tickets } from '$lib/api';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, fetch, parent }) => {
	const { user } = await parent();
	const isStaff = user?.role === 'admin' || user?.role === 'agent';
	try {
		const [ticket, comments, categories, staff, links] = await Promise.all([
			tickets.get(params.id, fetch),
			tickets.comments(params.id, fetch),
			// Only staff can change a ticket, so only they need the options.
			isStaff ? listCategories(fetch) : Promise.resolve([]),
			// Only staff whose groups cover this ticket can be assigned it.
			isStaff ? tickets.assignable(params.id, fetch) : Promise.resolve([]),
			isStaff ? tickets.links(params.id, fetch) : Promise.resolve(null)
		]);
		return { ticket, comments, categories, staff, links, isStaff };
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) error(404, 'Ticket not found');
		throw err;
	}
};
