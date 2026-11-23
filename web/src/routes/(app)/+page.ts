import { tickets } from '$lib/api';
import { ACTIVE_STATUSES } from '$lib/tickets';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	const [stats, recent] = await Promise.all([
		tickets.stats(fetch),
		tickets.list({ status: ACTIVE_STATUSES, limit: 6 }, fetch)
	]);
	return { stats, recent };
};
