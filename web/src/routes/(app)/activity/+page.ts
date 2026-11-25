import { error } from '@sveltejs/kit';
import { activity, type ActivityCategory, type ActivityQuery } from '$lib/api';
import type { PageLoad } from './$types';

const categories: ActivityCategory[] = ['auth', 'tickets', 'users', 'settings', 'knowledge', 'profile', 'reports', 'system'];

/** "2026-11-25" → local midnight, in Unix seconds. */
function dayStart(s: string | null): number | undefined {
	if (!s || !/^\d{4}-\d{2}-\d{2}$/.test(s)) return undefined;
	const [y, m, d] = s.split('-').map(Number);
	return Math.floor(new Date(y, m - 1, d).getTime() / 1000);
}

// Admins only. The page hides itself from everyone else; the API checks too.
export const load: PageLoad = async ({ url, fetch, parent }) => {
	const { user } = await parent();
	if (user?.role !== 'admin') error(404, 'Page not found');

	const p = url.searchParams;
	const category = p.get('category') as ActivityCategory | null;
	const from = p.get('from') ?? '';
	const to = p.get('to') ?? '';
	const toStart = dayStart(to);
	const query: ActivityQuery = {
		q: p.get('q')?.trim() ?? '',
		category: category && categories.includes(category) ? category : '',
		outcome: p.get('outcome') === 'failure' ? 'failure' : '',
		since: dayStart(from),
		// "To" includes that whole day.
		until: toStart === undefined ? undefined : toStart + 86400
	};
	return { query, from, to, ...(await activity.list(query, fetch)) };
};
