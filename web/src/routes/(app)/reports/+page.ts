import { error } from '@sveltejs/kit';
import { listCategories, reports } from '$lib/api';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ url, fetch, parent }) => {
	const { user } = await parent();
	if (user?.role !== 'admin' && user?.role !== 'agent') error(404, 'Page not found');
	const days = [7, 30, 90].includes(Number(url.searchParams.get('days'))) ? Number(url.searchParams.get('days')) : 30;
	const [report, categories] = await Promise.all([reports.get(days, fetch), listCategories(fetch)]);
	return { report, categories, days };
};
