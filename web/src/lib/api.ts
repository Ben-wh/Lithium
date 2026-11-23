import type { Background } from './backgrounds';
import type { CategoryColor } from './categories';

export type User = {
	id: number;
	email: string;
	name: string;
	role: 'admin' | 'agent' | 'user';
	/** "" when the user has no picture (show initials). */
	avatarUrl: string;
	createdAt: string;
	mustChangePassword: boolean;
	twoFactorEnabled: boolean;
	/**
	 * What this session still has to do: "verify" (enter a code after the
	 * password), "setup" (2FA is mandatory but not set up yet), or "".
	 */
	twoFactor: '' | 'verify' | 'setup';
};

export class ApiError extends Error {
	constructor(
		public status: number,
		message: string
	) {
		super(message);
	}
}

type Fetch = typeof fetch;

async function request<T>(path: string, init: RequestInit = {}, f: Fetch = fetch): Promise<T> {
	const isForm = init.body instanceof FormData;
	const res = await f(path, {
		credentials: 'same-origin',
		...init,
		// For FormData the browser sets multipart/form-data with its boundary.
		headers: isForm ? init.headers : { 'Content-Type': 'application/json', ...init.headers }
	});
	if (!res.ok) {
		let msg = res.statusText;
		try {
			msg = (await res.json()).error ?? msg;
		} catch {
			/* non-JSON error */
		}
		throw new ApiError(res.status, msg);
	}
	// Some endpoints succeed with no body (204, 202), so only parse when there is one.
	const text = await res.text();
	return (text ? JSON.parse(text) : undefined) as T;
}

export const me = (f?: Fetch) => request<User>('/api/auth/me', {}, f);

export const login = (email: string, password: string, remember: boolean) =>
	request<User>('/api/auth/login', {
		method: 'POST',
		body: JSON.stringify({ email, password, remember })
	});

// --- Two-factor authentication ---

export type TwoFactorStatus = { enabled: boolean; required: boolean; recoveryCodesLeft: number };
export type TwoFactorSetup = { secret: string; uri: string; qr: string };

export const twoFactor = {
	/** Finish signing in with an authenticator or recovery code. */
	verify: (code: string) =>
		request<User>('/api/auth/2fa/verify', { method: 'POST', body: JSON.stringify({ code }) }),
	status: () => request<TwoFactorStatus>('/api/auth/2fa'),
	/** A new secret and QR code; not active until enable() succeeds. */
	start: () => request<TwoFactorSetup>('/api/auth/2fa/setup', { method: 'POST', body: '{}' }),
	enable: (code: string) =>
		request<{ recoveryCodes: string[] }>('/api/auth/2fa/enable', { method: 'POST', body: JSON.stringify({ code }) }),
	newRecoveryCodes: (password: string) =>
		request<{ recoveryCodes: string[] }>('/api/auth/2fa/recovery-codes', {
			method: 'POST',
			body: JSON.stringify({ password })
		}),
	disable: (password: string) =>
		request<void>('/api/auth/2fa/disable', { method: 'POST', body: JSON.stringify({ password }) })
};

/** The sensitive mode banner; readable before signing in. */
export const publicBanner = (f?: Fetch) =>
	request<{ sensitiveMode: boolean; sensitiveMessage: string }>('/api/public/banner', {}, f);

export const logout = () => request<void>('/api/auth/logout', { method: 'POST', body: '{}' });

/** currentPassword is only needed when the change isn't forced. */
export const changePassword = (newPassword: string, currentPassword?: string) =>
	request<User>('/api/auth/password', {
		method: 'POST',
		body: JSON.stringify({ newPassword, currentPassword })
	});

export const requestPasswordReset = (email: string) =>
	request<void>('/api/auth/password-reset-request', {
		method: 'POST',
		body: JSON.stringify({ email })
	});

// --- Admin ---

export type Role = User['role'];
export type AdminUser = User & {
	pendingReset: boolean;
	/** Tickets they raised (deleted with the account). */
	ticketCount: number;
	/** Active tickets assigned to them. */
	assignedCount: number;
};

export type ResetRequest = {
	id: number;
	status: 'pending' | 'completed' | 'dismissed';
	ip: string;
	createdAt: string;
	resolvedAt?: string;
	user: { id: number; name: string; email: string };
};

export type NewUser = { name: string; email: string; role: Role; password: string };

export const admin = {
	listUsers: (q = '') => request<AdminUser[]>(`/api/admin/users?q=${encodeURIComponent(q)}`),
	createUser: (u: NewUser) =>
		request<User>('/api/admin/users', { method: 'POST', body: JSON.stringify(u) }),
	setPassword: (userId: number, password: string) =>
		request<void>(`/api/admin/users/${userId}/password`, {
			method: 'POST',
			body: JSON.stringify({ password })
		}),
	setRole: (userId: number, role: Role) =>
		request<User>(`/api/admin/users/${userId}`, { method: 'PATCH', body: JSON.stringify({ role }) }),
	deleteUser: (userId: number) =>
		request<{ ticketsDeleted: number }>(`/api/admin/users/${userId}`, { method: 'DELETE', body: '{}' }),
	/** Turn off someone's 2FA (e.g. lost phone) and sign them out everywhere. */
	resetTwoFactor: (userId: number) =>
		request<void>(`/api/admin/users/${userId}/2fa/reset`, { method: 'POST', body: '{}' }),
	listResetRequests: () => request<ResetRequest[]>('/api/admin/password-resets'),
	dismissResetRequest: (id: number) =>
		request<void>(`/api/admin/password-resets/${id}/dismiss`, { method: 'POST', body: '{}' })
};

// --- Tickets ---

export type CategoryRef = { id: number; name: string; color: CategoryColor };
export type PublicCategory = CategoryRef & { description: string };
export type Category = PublicCategory & { archived: boolean; ticketCount: number; createdAt: string };

/** Active categories, for anyone raising a ticket. */
export const listCategories = (f?: Fetch) => request<PublicCategory[]>('/api/categories', {}, f);

export const adminCategories = {
	list: () => request<Category[]>('/api/admin/categories'),
	create: (c: { name: string; description: string; color: CategoryColor }) =>
		request<Category>('/api/admin/categories', { method: 'POST', body: JSON.stringify(c) }),
	update: (id: number, c: Partial<{ name: string; description: string; color: CategoryColor; archived: boolean }>) =>
		request<Category>(`/api/admin/categories/${id}`, { method: 'PATCH', body: JSON.stringify(c) }),
	remove: (id: number) => request<void>(`/api/admin/categories/${id}`, { method: 'DELETE', body: '{}' })
};

export type UserRef = { id: number; name: string; email: string; avatarUrl: string };
export type Priority = 'low' | 'normal' | 'high' | 'urgent';
export type TicketStatus = 'unassigned' | 'in_progress' | 'pending' | 'resolved' | 'closed';

export type Ticket = {
	id: number;
	subject: string;
	description: string;
	status: TicketStatus;
	priority: Priority;
	requester: UserRef;
	createdBy: UserRef;
	assignee: UserRef | null;
	category: CategoryRef | null;
	attachments: Attachment[];
	createdAt: string;
	updatedAt: string;
};

export type Attachment = { id: number; filename: string; contentType: string; size: number; createdAt: string };
export const attachmentUrl = (a: Pick<Attachment, 'id'>) => `/api/attachments/${a.id}`;

/** Limits for images attached when raising a ticket (the server enforces the same). */
export const IMAGE_LIMITS = {
	maxFiles: 5,
	maxBytes: 10 * 1024 * 1024,
	types: ['image/png', 'image/jpeg', 'image/gif', 'image/webp']
} as const;

export type NewTicket = {
	subject: string;
	description: string;
	priority: Priority;
	categoryId?: number;
	/** Staff only: raise on someone else's behalf. */
	requesterId?: number;
};

export type TicketSummary = Omit<Ticket, 'description' | 'createdBy' | 'attachments'>;
export type TicketList = { tickets: TicketSummary[]; total: number };

export type TicketQuery = {
	/** Comma-separated statuses, e.g. "unassigned,in_progress". */
	status?: string;
	priority?: Priority | '';
	assignee?: 'me' | 'unassigned' | '';
	/** Category id. */
	category?: number | '';
	/** Staff only: one requester's tickets. */
	requester?: number;
	q?: string;
	limit?: number;
	offset?: number;
};

export type TicketStats = {
	unassigned: number;
	inProgress: number;
	pending: number;
	assignedToMe: number;
	resolvedThisWeek: number;
};

export const tickets = {
	create: (t: NewTicket, images: File[] = []) => {
		if (images.length === 0) {
			return request<Ticket>('/api/tickets', { method: 'POST', body: JSON.stringify(t) });
		}
		const form = new FormData();
		form.append('payload', JSON.stringify(t));
		for (const img of images) form.append('images', img, img.name);
		return request<Ticket>('/api/tickets', { method: 'POST', body: form });
	},
	get: (id: number | string, f?: Fetch) => request<Ticket>(`/api/tickets/${id}`, {}, f),
	list: (query: TicketQuery = {}, f?: Fetch) => {
		const params = new URLSearchParams();
		for (const [k, v] of Object.entries(query)) if (v !== undefined && v !== '') params.set(k, String(v));
		return request<TicketList>(`/api/tickets?${params}`, {}, f);
	},
	stats: (f?: Fetch) => request<TicketStats>('/api/tickets/stats', {}, f),
	/** Staff only. Only the fields given change; null clears category/assignee. */
	update: (id: number, changes: TicketChanges) =>
		request<Ticket>(`/api/tickets/${id}`, { method: 'PATCH', body: JSON.stringify(changes) }),
	comments: (id: number | string, f?: Fetch) => request<Comment[]>(`/api/tickets/${id}/comments`, {}, f),
	/** Staff only: who can be assigned this ticket (filtered by agent groups when they're on). */
	assignable: (id: number | string, f?: Fetch) => request<UserRef[]>(`/api/tickets/${id}/assignable`, {}, f),
	/** Staff only: a ticket's parent, children and incident root. */
	links: (id: number | string, f?: Fetch) => request<TicketLinks>(`/api/tickets/${id}/links`, {}, f),
	/** Staff only: link a ticket under another, or unlink it (null). Returns the ticket's links. */
	setParent: (id: number, parentId: number | null) =>
		request<TicketLinks>(`/api/tickets/${id}/parent`, { method: 'PUT', body: JSON.stringify({ parentId }) }),
	/** Staff only: every ticket in an incident tree (root first). */
	tree: (id: number | string, f?: Fetch) =>
		request<{ rootId: number; nodes: TicketLink[]; truncated: boolean }>(`/api/tickets/${id}/tree`, {}, f),
	addComment: (id: number, body: string, internal = false) =>
		request<Comment>(`/api/tickets/${id}/comments`, { method: 'POST', body: JSON.stringify({ body, internal }) }),
	/** Staff only: groups that can be @mentioned, with how many members can see the ticket. */
	mentionableGroups: (id: number, internal = false) =>
		request<{ id: number; name: string; memberCount: number }[]>(
			`/api/tickets/${id}/mentionable-groups${internal ? '?internal=1' : ''}`
		),
	/** People who can be @mentioned on a ticket (staff only for internal notes). */
	mentionable: (id: number, internal = false) =>
		request<UserRef[]>(`/api/tickets/${id}/mentionable${internal ? '?internal=1' : ''}`)
};

export type TicketLink = {
	id: number;
	/** Set in incident trees (0 for the root). */
	parentId?: number;
	/** In an incident tree: a ticket outside the viewer's groups; only its position is known. */
	restricted?: boolean;
	subject: string;
	status: TicketStatus;
	priority: Priority;
	assignee: UserRef | null;
	category: CategoryRef | null;
};

export type TicketLinks = {
	parent: TicketLink | null;
	children: TicketLink[];
	/** Top of this ticket's tree when it's urgent (and the incident tree is on); 0 otherwise. */
	incidentRootId: number;
};

export type TicketChanges = Partial<{
	status: TicketStatus;
	priority: Priority;
	categoryId: number | null;
	assigneeId: number | null;
}>;

export type TicketEvent = 'status' | 'priority' | 'category' | 'assignee' | 'parent' | 'child';

/**
 * One entry in a ticket's conversation: a reply (kind "comment"), or an
 * automatic Lithium update (kind "event") where author is who made the change.
 * For status/priority events from/to are keys; for category/assignee they're
 * names at the time ("" = none).
 */
export type Comment = {
	id: number;
	kind: 'comment' | 'event';
	author: { id: number; name: string; role: Role; avatarUrl: string } | null;
	body?: string;
	/** Staff-only note, never shown to the requester. */
	internal: boolean;
	event?: TicketEvent;
	from: string;
	to: string;
	createdAt: string;
};

/** Staff only: look users up by name or email. */
export const searchUsers = (q: string, opts: { staff?: boolean } = {}, f?: Fetch) =>
	request<UserRef[]>(`/api/users?q=${encodeURIComponent(q)}${opts.staff ? '&staff=1' : ''}`, {}, f);

// --- Workspace settings ---

export type Settings = {
	/** Choosing a category is mandatory when raising a ticket (while any active category exists). */
	requireCategory: boolean;
	/** Users can attach images when raising a ticket. */
	allowImageUploads: boolean;
	/** Shown before ticket numbers: "TID" → "TID-42". */
	ticketPrefix: string;
	/** Users can't upload their own profile picture (the built-in ones still work). */
	disableCustomAvatars: boolean;
	/** Regular users (not just staff) can @mention people. */
	allowUserMentions: boolean;
	/** Character limits for new tickets. */
	subjectMaxLength: number;
	descriptionMaxLength: number;
	/** Staff can view a diagram of tickets linked under an urgent ticket. */
	incidentTree: boolean;
	/** Agents only see tickets in their groups' categories (plus uncategorised and their own). */
	agentGroups: boolean;
	/** Workspace background pattern, chosen by an admin. */
	background: Background;
	/** Show a red banner with sensitiveMessage above the navbar on every page. */
	sensitiveMode: boolean;
	sensitiveMessage: string;
	/** Everyone must set up an authenticator app before using Lithium. */
	requireTwoFactor: boolean;
};

/** Ranges admins can set the ticket length limits to (the server enforces the same). */
export const LENGTH_LIMITS = {
	subject: { min: 20, max: 200 },
	description: { min: 100, max: 20000 }
} as const;

export const getSettings = (f?: Fetch) => request<Settings>('/api/settings', {}, f);

export const adminSettings = {
	update: (s: Partial<Settings>) =>
		request<Settings>('/api/admin/settings', { method: 'PATCH', body: JSON.stringify(s) })
};

// --- Profile ---

/** How many built-in avatars ship in static/avatars (1.svg … N.svg). */
export const PRESET_AVATARS = 10;
export const presetAvatarUrl = (n: number) => `/avatars/${n}.svg`;
export const MAX_AVATAR_BYTES = 2 * 1024 * 1024;

export const profile = {
	/** A built-in avatar (1–10), or 0 to go back to initials. */
	setPresetAvatar: (preset: number) =>
		request<User>('/api/profile/avatar', { method: 'PUT', body: JSON.stringify({ preset }) }),
	uploadAvatar: (file: File) => {
		const form = new FormData();
		form.append('image', file, file.name);
		return request<User>('/api/profile/avatar', { method: 'POST', body: form });
	}
};

// --- Notifications ---

export type Notification = {
	id: number;
	kind: 'reply' | 'note' | 'mention' | 'update' | 'assigned';
	/** What the actor did, e.g. "changed the status to Resolved". */
	summary: string;
	actor: UserRef | null;
	ticket: { id: number; subject: string };
	read: boolean;
	createdAt: string;
};

export const notificationsApi = {
	list: () => request<{ items: Notification[]; unread: number }>('/api/notifications'),
	unread: () => request<{ unread: number }>('/api/notifications/unread'),
	/** Mark specific notifications, all for one ticket, or (with neither) everything as read. */
	markRead: (opts: { ids?: number[]; ticketId?: number } = {}) =>
		request<{ unread: number }>('/api/notifications/read', { method: 'POST', body: JSON.stringify(opts) })
};

// --- Agent groups (admin) ---

export type Group = {
	id: number;
	name: string;
	description: string;
	members: UserRef[];
	categories: CategoryRef[];
	createdAt: string;
};

export type GroupInput = { name: string; description: string; memberIds: number[]; categoryIds: number[] };

export const adminGroups = {
	list: () => request<Group[]>('/api/admin/groups'),
	create: (g: GroupInput) => request<Group>('/api/admin/groups', { method: 'POST', body: JSON.stringify(g) }),
	update: (id: number, g: GroupInput) =>
		request<Group>(`/api/admin/groups/${id}`, { method: 'PUT', body: JSON.stringify(g) }),
	remove: (id: number) => request<void>(`/api/admin/groups/${id}`, { method: 'DELETE', body: '{}' })
};

// --- User book (staff) ---

/** Someone who raises tickets, with counts of the tickets the viewer can see. */
export type Person = UserRef & {
	role: Role;
	openTickets: number;
	totalTickets: number;
	lastTicketAt: string | null;
	createdAt: string;
};

export const people = {
	list: (q = '', offset = 0, f?: Fetch) =>
		request<{ people: Person[]; total: number }>(`/api/people?q=${encodeURIComponent(q)}&offset=${offset}`, {}, f),
	get: (id: number | string, f?: Fetch) => request<Person>(`/api/people/${id}`, {}, f)
};

// --- Knowledge base ---

export type KBCollection = { id: number; name: string; description: string; articleCount: number };
export type KBVisibility = 'staff' | 'everyone';
export type KBStatus = 'draft' | 'published';

export type KBArticle = {
	id: number;
	title: string;
	/** Markdown. Left out of lists. */
	body?: string;
	/** Plain-text preview, in lists. */
	excerpt?: string;
	collection: { id: number; name: string } | null;
	visibility: KBVisibility;
	status: KBStatus;
	author: UserRef | null;
	updatedBy: UserRef | null;
	createdAt: string;
	updatedAt: string;
	publishedAt: string | null;
};

export type KBArticleInput = {
	title: string;
	body: string;
	collectionId: number;
	visibility: KBVisibility;
	status: KBStatus;
};

export const kb = {
	collections: (f?: Fetch) => request<KBCollection[]>('/api/kb/collections', {}, f),
	createCollection: (c: { name: string; description: string }) =>
		request<KBCollection>('/api/kb/collections', { method: 'POST', body: JSON.stringify(c) }),
	updateCollection: (id: number, c: { name: string; description: string }) =>
		request<KBCollection>(`/api/kb/collections/${id}`, { method: 'PUT', body: JSON.stringify(c) }),
	deleteCollection: (id: number) => request<void>(`/api/kb/collections/${id}`, { method: 'DELETE', body: '{}' }),

	articles: (q: { q?: string; collection?: number | 'none'; status?: KBStatus } = {}, f?: Fetch) => {
		const params = new URLSearchParams();
		for (const [k, v] of Object.entries(q)) if (v !== undefined && v !== '') params.set(k, String(v));
		return request<KBArticle[]>(`/api/kb/articles?${params}`, {}, f);
	},
	article: (id: number | string, f?: Fetch) => request<KBArticle>(`/api/kb/articles/${id}`, {}, f),
	/** Starts a new draft. */
	create: (collectionId = 0) =>
		request<KBArticle>('/api/kb/articles', { method: 'POST', body: JSON.stringify({ collectionId }) }),
	save: (id: number, a: KBArticleInput) =>
		request<KBArticle>(`/api/kb/articles/${id}`, { method: 'PUT', body: JSON.stringify(a) }),
	remove: (id: number) => request<void>(`/api/kb/articles/${id}`, { method: 'DELETE', body: '{}' }),
	uploadImage: (id: number, file: File) => {
		const form = new FormData();
		form.append('image', file, file.name);
		return request<{ url: string; name: string }>(`/api/kb/articles/${id}/images`, { method: 'POST', body: form });
	}
};

// --- Reports (staff) ---

export type NamedCount = { key: string; name: string; color?: string; avatarUrl?: string; count: number };

export type Report = {
	days: number;
	series: { date: string; raised: number; resolved: number }[];
	totals: {
		raised: number;
		resolved: number;
		open: number;
		medianFirstResponseHours: number | null;
		medianResolutionHours: number | null;
	};
	byStatus: NamedCount[];
	byPriority: NamedCount[];
	byCategory: NamedCount[];
	byAssignee: NamedCount[];
};

const tz = () => new Date().getTimezoneOffset();

export const reports = {
	get: (days: number, f?: Fetch) => request<Report>(`/api/reports?days=${days}&tz=${tz()}`, {}, f),
	/** URL that downloads matching tickets as a file. */
	exportUrl: (o: { from: string; to: string; status?: string; category?: number | ''; description?: boolean; format: 'csv' | 'json' }) => {
		const p = new URLSearchParams({ from: o.from, to: o.to, format: o.format, tz: String(tz()) });
		if (o.status) p.set('status', o.status);
		if (o.category) p.set('category', String(o.category));
		if (o.description) p.set('description', '1');
		return `/api/reports/export?${p}`;
	}
};
