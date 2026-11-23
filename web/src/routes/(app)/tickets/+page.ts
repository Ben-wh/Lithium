import { listCategories, tickets, type Priority, type TicketQuery } from '$lib/api';
import { TICKET_PAGE_SIZE, ticketViews, type TicketView } from '$lib/tickets';
import type { PageLoad } from './$types';

const priorities = new Set(['low', 'normal', 'high', 'urgent']);
const assignees = new Set(['me', 'unassigned']);

// Filters live in the URL so Back and shared links work. Unknown values are
// ignored rather than sent to the server.
export const load: PageLoad = async ({ url, fetch }) => {
	const p = url.searchParams;
	const pick = <T extends string>(key: string, allowed: Set<string>, fallback: T) =>
		(allowed.has(p.get(key) ?? '') ? p.get(key) : fallback) as T;

	const view = pick<TicketView>('view', new Set(Object.keys(ticketViews)), 'active');
	const priority = pick<Priority | ''>('priority', priorities, '');
	const assignee = pick<'me' | 'unassigned' | ''>('assignee', assignees, '');
	const categoryParam = Number(p.get('category'));
	const category: number | '' = Number.isInteger(categoryParam) && categoryParam > 0 ? categoryParam : '';
	const q = p.get('q')?.trim() ?? '';

	const query: TicketQuery = { status: ticketViews[view].status, priority, assignee, category, q, limit: TICKET_PAGE_SIZE };
	const [list, categories] = await Promise.all([tickets.list(query, fetch), listCategories(fetch)]);
	return { list, categories, query, filters: { view, priority, assignee, category, q } };
};
