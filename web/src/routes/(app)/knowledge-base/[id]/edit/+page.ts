import { error } from '@sveltejs/kit';
import { ApiError, kb } from '$lib/api';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, fetch, parent }) => {
	const { user } = await parent();
	if (user?.role !== 'admin' && user?.role !== 'agent') error(404, 'Article not found');
	try {
		const [article, collections] = await Promise.all([kb.article(params.id, fetch), kb.collections(fetch)]);
		return { article, collections };
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) error(404, 'Article not found');
		throw err;
	}
};
