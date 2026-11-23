import type { Priority, TicketStatus } from './api';

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
	closed: 'Closed'
};

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
	all: { label: 'All', status: '' }
} as const;
export type TicketView = keyof typeof ticketViews;

export const TICKET_PAGE_SIZE = 50;

/** One-line explanations shown in the status picker. */
export const statusHints: Record<TicketStatus, string> = {
	unassigned: 'Waiting for someone to pick it up',
	in_progress: 'Being worked on',
	pending: 'Waiting on the requester or someone else',
	resolved: 'Fixed, pending confirmation',
	closed: 'Done, no further action'
};

/** A ticket's reference as people see it, e.g. "TID-42". */
export const ticketRef = (id: number, prefix = 'TID') => `${prefix}-${id}`;
