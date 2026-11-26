import type { Priority, TicketSort, TicketStatus } from './api';

export const priorities: { value: Priority; label: string; hint: string }[] = [
	{ value: 'low', label: 'Low', hint: 'Minor, no rush' },
	{ value: 'normal', label: 'Normal', hint: 'Needs doing, not blocking' },
	{ value: 'high', label: 'High', hint: 'Blocking some work' },
	{ value: 'urgent', label: 'Urgent', hint: 'Something critical is down' }
];

export const priorityLabel = (p: Priority) => priorities.find((x) => x.value === p)?.label ?? p;

export const statusLabels: Record<TicketStatus, string> = {
	unassigned: 'Unassigned',
	in_progress: 'In progress',
	pending: 'Pending',
	resolved: 'Resolved',
	closed: 'Closed',
	raised_in_error: 'Raised in error'
};

/**
 * Each status's colour, as a category tag class (see app.css) so it works in
 * light and dark. The reports chart uses the same hues.
 */
export const statusTags: Record<TicketStatus, string> = {
	unassigned: 'tag-blue',
	in_progress: 'tag-orange',
	pending: 'tag-amber',
	resolved: 'tag-green',
	closed: 'tag-gray',
	raised_in_error: 'tag-red'
};

/** A finished ticket, which its requester or staff can reopen. */
export const isDone = (s: TicketStatus) => s === 'resolved' || s === 'closed' || s === 'raised_in_error';

export const initials = (name: string) =>
	name
		.split(/\s+/)
		.filter(Boolean)
		.slice(0, 2)
		.map((w) => w[0])
		.join('')
		.toUpperCase();

/** Statuses that still need work. */
export const ACTIVE_STATUSES = 'unassigned,in_progress,pending';

/** Status tabs on the tickets list. */
export const ticketViews = {
	active: { label: 'Active', status: ACTIVE_STATUSES },
	unassigned: { label: 'Unassigned', status: 'unassigned' },
	in_progress: { label: 'In progress', status: 'in_progress' },
	pending: { label: 'Pending', status: 'pending' },
	resolved: { label: 'Resolved', status: 'resolved' },
	closed: { label: 'Closed', status: 'closed' },
	raised_in_error: { label: 'Raised in error', status: 'raised_in_error' },
	all: { label: 'All', status: '' }
} as const;
export type TicketView = keyof typeof ticketViews;

export const TICKET_PAGE_SIZE = 50;

/**
 * Sort choices on the tickets list, as "key-order" (the value kept in the
 * URL). The first is the default.
 */
export const ticketSorts = [
	{ value: 'updated-desc', label: 'Recently updated' },
	{ value: 'updated-asc', label: 'Least recently updated' },
	{ value: 'created-desc', label: 'Newest first' },
	{ value: 'created-asc', label: 'Oldest first' },
	{ value: 'priority-desc', label: 'Priority: highest first' },
	{ value: 'priority-asc', label: 'Priority: lowest first' },
	{ value: 'status-asc', label: 'Status' },
	{ value: 'subject-asc', label: 'Subject: A to Z' },
	{ value: 'subject-desc', label: 'Subject: Z to A' },
	{ value: 'assignee-asc', label: 'Assignee: A to Z', staffOnly: true }
] as const satisfies readonly { value: `${TicketSort}-${'asc' | 'desc'}`; label: string; staffOnly?: boolean }[];
export type TicketSortChoice = (typeof ticketSorts)[number]['value'];

/** One-line explanations shown in the status picker. */
export const statusHints: Record<TicketStatus, string> = {
	unassigned: 'Waiting for someone to pick it up',
	in_progress: 'Being worked on',
	pending: 'Waiting on the requester or someone else',
	resolved: 'Fixed, pending confirmation',
	closed: 'Done, no further action',
	raised_in_error: 'Not needed: a duplicate, mistake or test'
};

/** A ticket's reference as people see it, e.g. "TID-42". */
export const ticketRef = (id: number, prefix = 'TID') => `${prefix}-${id}`;
