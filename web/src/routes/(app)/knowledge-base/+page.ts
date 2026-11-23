import { kb, type KBStatus } from '$lib/api';
import type { PageLoad } from './$types';

// Filters live in the URL: ?collection=<id>|none, ?status=draft (staff), ?q=.
export const load: PageLoad = async ({ url, fetch }) => {
	const p = url.searchParams;
	const raw = p.get('collection');
	const collection: number | 'none' | '' = raw === 'none' ? 'none' : Number(raw) > 0 ? Number(raw) : '';
	const status: KBStatus | '' = p.get('status') === 'draft' ? 'draft' : '';
	const q = p.get('q')?.trim() ?? '';
	const [collections, articles] = await Promise.all([
		kb.collections(fetch),
		kb.articles({ q, collection: collection || undefined, status: status || undefined }, fetch)
	]);
	return { collections, articles, filters: { collection, status, q } };
};
