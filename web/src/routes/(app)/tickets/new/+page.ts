import { listCategories, people, type UserRef } from '$lib/api';
import type { PageLoad } from './$types';

// Workspace settings come from the (app) layout. Staff can arrive with
// ?requester=<id> (e.g. from the User book) to raise a ticket for someone.
export const load: PageLoad = async ({ fetch, url, parent }) => {
	const { user } = await parent();
	const isStaff = user?.role === 'admin' || user?.role === 'agent';
	const requesterId = Number(url.searchParams.get('requester'));
	const [categories, preset] = await Promise.all([
		listCategories(fetch),
		isStaff && requesterId > 0 ? people.get(requesterId, fetch).catch(() => null) : Promise.resolve(null)
	]);
	const presetRequester: UserRef | null = preset
		? { id: preset.id, name: preset.name, email: preset.email, avatarUrl: preset.avatarUrl }
		: null;
	return { categories, presetRequester };
};
