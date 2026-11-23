import { error } from '@sveltejs/kit';
import { ApiError, kb } from '$lib/api';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params, fetch }) => {
	try {
		return { article: await kb.article(params.id, fetch) };
	} catch (err) {
		if (err instanceof ApiError && err.status === 404) error(404, 'Article not found');
		throw err;
	}
};
