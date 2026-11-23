<script lang="ts">
	import { page } from '$app/state';
	import { replaceState } from '$app/navigation';
	import Icon from '$lib/components/Icon.svelte';
	import Avatar from '$lib/components/Avatar.svelte';
	import PriorityIcon from '$lib/components/PriorityIcon.svelte';
	import CategorySelect from '$lib/components/CategorySelect.svelte';
	import Dropdown, { type DropdownOption } from '$lib/components/Dropdown.svelte';
	import CategoryTag from '$lib/components/CategoryTag.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import TicketTimeline from '$lib/components/TicketTimeline.svelte';
	import LinkedTickets from '$lib/components/LinkedTickets.svelte';
	import MentionTextarea, { type MentionCandidate } from '$lib/components/MentionTextarea.svelte';
	import { encodeMentions } from '$lib/mentions';
	import { notifications } from '$lib/notifications.svelte';
	import {
		ApiError,
		attachmentUrl,
		tickets,
		type Comment,
		type PublicCategory,
		type Ticket,
		type TicketChanges
	} from '$lib/api';
	import { dateTimeFmt, formatBytes } from '$lib/format';
	import { priorities, priorityLabel, statusHints, statusLabels, ticketRef } from '$lib/tickets';

	let { data } = $props();

	// Local copies so changes and replies show without reloading the page.
	let t = $derived<Ticket>(data.ticket);
	let comments = $derived<Comment[]>(data.comments);
	let links = $derived(data.links);
	// Staff who can be assigned; refreshed after changes, since changing the
	// category can change who's eligible.
	let assignable = $derived(data.staff);
	const canAssignMe = $derived(assignable.some((u) => u.id === me));
	const me = $derived(data.user?.id);
	const ref = $derived(ticketRef(t.id, data.settings.ticketPrefix));

	// Show a one-off confirmation after raising, then tidy the URL.
	let justCreated = $state(page.url.searchParams.has('created'));
	$effect(() => {
		if (justCreated) replaceState(page.url.pathname, page.state);
	});

	const errorText = (err: unknown) =>
		err instanceof ApiError && err.status < 500
			? err.message.charAt(0).toUpperCase() + err.message.slice(1) + '.'
			: 'Something went wrong. Please try again.';

	// --- Staff changes ---
	let saving = $state(false);
	let changeError = $state('');

	async function change(changes: TicketChanges) {
		saving = true;
		changeError = '';
		try {
			t = await tickets.update(t.id, changes);
			[comments, assignable] = await Promise.all([tickets.comments(t.id), tickets.assignable(t.id)]); // pick up Lithium's updates
		} catch (err) {
			changeError = errorText(err);
		} finally {
			saving = false;
		}
	}

	// An archived category isn't offered for new choices, but keep showing it
	// if the ticket already has it.
	const categoryOptions = $derived.by<PublicCategory[]>(() => {
		const opts = data.categories;
		const cur = t.category;
		return cur && !opts.some((c) => c.id === cur.id) ? [...opts, { ...cur, description: '' }] : opts;
	});
	const staffList = $derived.by(() => {
		const a = t.assignee;
		return a && !assignable.some((u) => u.id === a.id) ? [...assignable, a] : assignable;
	});
	const staffById = $derived(new Map(staffList.map((u) => [u.id, u])));
	const assigneeOptions = $derived<DropdownOption<number | ''>[]>([
		{ value: '', label: 'Unassigned' },
		// You first, then everyone else alphabetically (as the server returns them).
		...[...staffList]
			.sort((a, b) => Number(b.id === me) - Number(a.id === me))
			.map((u) => ({ value: u.id, label: u.name, keywords: u.email }))
	]);
	const statusOptions = (Object.keys(statusLabels) as Ticket['status'][]).map((s) => ({ value: s, label: statusLabels[s] }));
	const priorityOptions = priorities.map((p) => ({ value: p.value, label: p.label }));

	// Opening a ticket marks its notifications as read.
	$effect(() => {
		notifications.markTicketRead(data.ticket.id);
	});

	// --- Replies ---
	let reply = $state('');
	let internal = $state(false); // staff-only note
	let sending = $state(false);
	let replyError = $state('');
	// "@Name" picked from suggestions, turned into stored mentions on send.
	let mentioned = new Map<string, string>();
	// Internal notes can only mention staff, so they get their own list.
	// People who can see the ticket, then (for staff) groups.
	const loadMentionable = $derived(async (): Promise<MentionCandidate[]> => {
		const [people, groups] = await Promise.all([
			tickets.mentionable(t.id, internal),
			data.isStaff ? tickets.mentionableGroups(t.id, internal) : Promise.resolve([])
		]);
		return [
			...people.map((p) => ({ kind: 'user' as const, target: String(p.id), name: p.name, email: p.email, avatarUrl: p.avatarUrl })),
			...groups.map((g) => ({ kind: 'group' as const, target: `g:${g.id}`, name: g.name, memberCount: g.memberCount }))
		];
	});
	// Staff can always mention; regular users only if an admin allows it.
	const canMention = $derived(data.isStaff || data.settings.allowUserMentions);
	const mentionHint = $derived(canMention ? ' Type @ to mention someone.' : '');

	async function sendReply(e?: SubmitEvent) {
		e?.preventDefault();
		if (sending) return;
		if (!reply.trim()) {
			replyError = 'Write a reply first.';
			return;
		}
		sending = true;
		replyError = '';
		try {
			const c = await tickets.addComment(t.id, encodeMentions(reply, mentioned), internal);
			comments = [...comments, c];
			reply = '';
			mentioned = new Map();
		} catch (err) {
			replyError = errorText(err);
		} finally {
			sending = false;
		}
	}

	function onReplyKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
			e.preventDefault();
			sendReply();
		}
	}

	const dt = 'text-xs text-ink-500';
	const dd = 'mt-1 flex items-center gap-2 text-sm text-ink-100';
</script>

<svelte:head>
	<title>{ref} {t.subject} · Lithium</title>
</svelte:head>

<div class="mx-auto max-w-5xl px-4 py-10 sm:px-6">
	<a href="/tickets" class="inline-flex items-center gap-1.5 text-sm text-ink-400 transition hover:text-ink-100">
		<Icon name="arrowLeft" class="size-4" /> Tickets
	</a>

	{#if justCreated}
		<div role="status" class="mt-4 flex items-center gap-2.5 rounded-lg border border-ink-700 bg-ink-900 px-3.5 py-2.5 text-sm text-ink-200">
			<Icon name="check" class="size-4 shrink-0 text-primary-300" />
			<span class="flex-1">
				Ticket raised.{t.requester.id === me ? " We'll keep you posted here." : ` ${t.requester.name} can now see it.`}
			</span>
			<button type="button" onclick={() => (justCreated = false)} aria-label="Dismiss" class="text-ink-500 hover:text-ink-100">
				<Icon name="close" class="size-4" />
			</button>
		</div>
	{/if}

	<div class="mt-4 flex flex-wrap items-center gap-2">
		<span class="font-mono text-sm text-ink-500">{ref}</span>
		<StatusBadge status={t.status} />
	</div>
	<h1 class="mt-2 text-2xl font-semibold tracking-tight break-words">{t.subject}</h1>

	<div class="mt-8 grid gap-6 lg:grid-cols-[1fr_300px]">
		<div class="min-w-0 space-y-6">
			<!-- Original post -->
			<article class="rounded-xl border border-ink-800 bg-ink-900/60">
				<header class="flex items-center gap-3 border-b border-ink-800 px-5 py-3.5">
					<Avatar name={t.createdBy.name} src={t.createdBy.avatarUrl} size={32} />
					<div class="min-w-0 text-sm">
						<p class="truncate">
							<span class="font-medium text-ink-100">{t.createdBy.name}</span>
							<span class="text-ink-500">
								{t.createdBy.id === t.requester.id ? 'raised this' : `raised this for ${t.requester.name}`}
							</span>
						</p>
						<p class="text-xs text-ink-500">{dateTimeFmt.format(new Date(t.createdAt))}</p>
					</div>
				</header>
				<div class="px-5 py-4 text-sm leading-relaxed whitespace-pre-wrap text-ink-200 break-words">{t.description}</div>
				{#if t.attachments.length > 0}
					<div class="border-t border-ink-800 px-5 py-4">
						<p class="mb-2.5 flex items-center gap-1.5 text-xs font-medium text-ink-400">
							<Icon name="image" class="size-3.5" />
							{t.attachments.length} {t.attachments.length === 1 ? 'image' : 'images'}
						</p>
						<ul class="grid grid-cols-3 gap-2 sm:grid-cols-5">
							{#each t.attachments as a (a.id)}
								<li>
									<a
										href={attachmentUrl(a)}
										target="_blank"
										rel="noopener"
										title="{a.filename} · {formatBytes(a.size)}"
										class="block aspect-square overflow-hidden rounded-lg border border-ink-800 bg-ink-950 transition hover:border-primary-400/60 focus-visible:ring-2 focus-visible:ring-primary-400/50 focus-visible:outline-none"
									>
										<img src={attachmentUrl(a)} alt={a.filename} loading="lazy" class="size-full object-cover" />
									</a>
								</li>
							{/each}
						</ul>
					</div>
				{/if}
			</article>

			<!-- Conversation -->
			<section aria-labelledby="activity-heading">
				<h2 id="activity-heading" class="mb-4 text-sm font-semibold">
					Activity
					{#if comments.length}<span class="font-normal text-ink-500">· {comments.length}</span>{/if}
				</h2>
				{#if comments.length > 0}
					<TicketTimeline {comments} requesterId={t.requester.id} />
				{:else}
					<p class="text-sm text-ink-500">No replies yet.</p>
				{/if}

				<form
					onsubmit={sendReply}
					class={[
						'mt-5 rounded-xl border p-3 transition',
						internal ? 'tag-amber internal-note border-dashed' : 'border-ink-800 bg-ink-900/60 focus-within:border-primary-400/50'
					]}
				>
					{#if data.isStaff}
						<div role="tablist" aria-label="Reply type" class="mb-2 flex gap-1">
							<button
								type="button"
								role="tab"
								aria-selected={!internal}
								onclick={() => (internal = false)}
								class={['rounded-md px-2.5 py-1 text-xs font-medium transition', !internal ? 'bg-ink-850 text-ink-100' : 'text-ink-500 hover:text-ink-200']}
							>
								Reply
							</button>
							<button
								type="button"
								role="tab"
								aria-selected={internal}
								onclick={() => (internal = true)}
								class={['inline-flex items-center gap-1 rounded-md px-2.5 py-1 text-xs font-medium transition', internal ? 'tag-chip' : 'text-ink-500 hover:text-ink-200']}
							>
								<Icon name="lock" class="size-3" /> Internal note
							</button>
						</div>
					{/if}
					<label for="reply" class="sr-only">{internal ? 'Internal note' : 'Reply'}</label>
					<MentionTextarea
						id="reply"
						bind:value={reply}
						load={loadMentionable}
						enabled={canMention}
						onmention={(name, id) => mentioned.set(name, id)}
						onkeydown={onReplyKeydown}
						maxlength={10000}
						placeholder={internal
							? `Only staff will see this.${mentionHint}`
							: data.isStaff && t.requester.id !== me
								? `Reply to ${t.requester.name.split(' ')[0]}…${mentionHint}`
								: `Write a reply…${mentionHint}`}
						class="block min-h-20 w-full resize-y bg-transparent px-1 py-1 text-sm leading-relaxed text-ink-100 outline-none placeholder:text-ink-500"
					/>
					<div class="mt-2 flex items-center justify-between gap-3">
						{#if replyError}
							<p role="alert" class="text-xs text-primary-300">{replyError}</p>
						{:else if internal}
							<p class="text-xs text-ink-400">Hidden from {t.requester.name.split(' ')[0]}. They won't be notified.</p>
						{:else}
							<p class="hidden text-xs text-ink-500 sm:block">
								<kbd class="rounded border border-ink-700 px-1 font-sans">⌘</kbd>
								<kbd class="rounded border border-ink-700 px-1 font-sans">Enter</kbd> to send
							</p>
						{/if}
						<button
							type="submit"
							disabled={sending || !reply.trim()}
							class="ml-auto inline-flex h-8 items-center rounded-lg bg-primary-400 px-3.5 text-[13px] font-semibold text-primary-950 transition hover:bg-primary-300 disabled:cursor-not-allowed disabled:opacity-50"
						>
							{sending ? 'Sending…' : internal ? 'Add note' : 'Reply'}
						</button>
					</div>
				</form>
			</section>
		</div>

		<!-- Details -->
		<aside class="rounded-xl border border-ink-800 bg-ink-900/60 p-5 lg:sticky lg:top-24 lg:self-start">
			{#if data.isStaff}
				<div class="space-y-4">
					{#if changeError}
						<p role="alert" class="rounded-lg border border-primary-400/25 bg-primary-400/10 px-3 py-2 text-xs text-primary-200">{changeError}</p>
					{/if}

					<div>
						<div class="mb-1 flex items-center justify-between">
							<label for="assignee" class={dt}>Assignee</label>
							{#if me && t.assignee?.id !== me && canAssignMe}
								<button
									type="button"
									disabled={saving}
									onclick={() => change({ assigneeId: me })}
									class="text-xs font-medium text-primary-300 hover:text-primary-200 disabled:opacity-50"
								>
									Assign to me
								</button>
							{/if}
						</div>
						{#key t.assignee?.id}
							<Dropdown
								id="assignee"
								size="sm"
								value={t.assignee?.id ?? ''}
								options={assigneeOptions}
								searchable={staffList.length > 6}
								disabled={saving}
								onchange={(v) => change({ assigneeId: v === '' ? null : v })}
							>
								{#snippet item(opt)}
									{@const u = opt.value === '' ? null : staffById.get(opt.value)}
									{#if u}
										<Avatar name={u.name} src={u.avatarUrl} size={24} tone={u.id === me ? 'soft' : 'neutral'} />
										<span class="min-w-0 truncate text-sm text-ink-100">
											{u.name}{#if u.id === me}<span class="ml-1 text-ink-500">(you)</span>{/if}
										</span>
									{:else}
										<span class="size-6 shrink-0 rounded-full border border-dashed border-ink-500"></span>
										<span class="text-sm text-ink-400">Unassigned</span>
									{/if}
								{/snippet}
							</Dropdown>
						{/key}
					</div>

					<div>
						<label for="status" class="mb-1 block {dt}">Status</label>
						{#key t.status}
							<Dropdown
								id="status"
								size="sm"
								value={t.status}
								options={statusOptions}
								disabled={saving}
								onchange={(v) => change({ status: v })}
							>
								{#snippet item(opt)}
									<span class="min-w-0">
										<StatusBadge status={opt.value} />
										<span class="mt-1 block text-xs text-ink-500">{statusHints[opt.value]}</span>
									</span>
								{/snippet}
								{#snippet selected(opt)}
									<StatusBadge status={opt.value} />
								{/snippet}
							</Dropdown>
						{/key}
					</div>

					<div>
						<label for="priority" class="mb-1 block {dt}">Priority</label>
						{#key t.priority}
							<Dropdown
								id="priority"
								size="sm"
								value={t.priority}
								options={priorityOptions}
								disabled={saving}
								onchange={(v) => change({ priority: v })}
							>
								{#snippet item(opt)}
									<PriorityIcon priority={opt.value} class="size-4 shrink-0 text-ink-400" />
									<span class="min-w-0">
										<span class="block text-sm text-ink-100">{opt.label}</span>
										<span class="block text-xs text-ink-500">{priorities.find((p) => p.value === opt.value)?.hint}</span>
									</span>
								{/snippet}
								{#snippet selected(opt)}
									<PriorityIcon priority={opt.value} class="size-4 shrink-0 text-ink-400" />
									<span class="text-ink-100">{opt.label}</span>
								{/snippet}
							</Dropdown>
						{/key}
					</div>

					<div>
						<label for="category" class="mb-1 block {dt}">Category</label>
						{#key t.category?.id}
							<CategorySelect
								id="category"
								value={t.category?.id ?? ''}
								options={categoryOptions}
								size="sm"
								disabled={saving}
								onchange={(v) => change({ categoryId: v === '' ? null : v })}
							/>
						{/key}
					</div>

					{#if links}
						<div class="border-t border-ink-800 pt-4">
							<LinkedTickets
								ticketId={t.id}
								bind:links
								onchange={async () => (comments = await tickets.comments(t.id))}
							/>
						</div>
					{/if}

					<div class="border-t border-ink-800 pt-4">
						<p class={dt}>Requester</p>
						<p class={dd}>
							<Avatar name={t.requester.name} src={t.requester.avatarUrl} size={24} />
							<span class="truncate">{t.requester.name}</span>
						</p>
					</div>
					<div>
						<p class={dt}>Created</p>
						<p class={dd}>{dateTimeFmt.format(new Date(t.createdAt))}</p>
					</div>
				</div>
			{:else}
				<dl class="space-y-4">
					<div>
						<dt class={dt}>Status</dt>
						<dd class={dd}><StatusBadge status={t.status} /></dd>
					</div>
					<div>
						<dt class={dt}>Assignee</dt>
						<dd class={dd}>
							{#if t.assignee}{t.assignee.name}{:else}<span class="text-ink-500">Not picked up yet</span>{/if}
						</dd>
					</div>
					<div>
						<dt class={dt}>Category</dt>
						<dd class={dd}>
							{#if t.category}
								<CategoryTag category={t.category} class="text-xs" />
							{:else}
								<span class="text-ink-500">None</span>
							{/if}
						</dd>
					</div>
					<div>
						<dt class={dt}>Priority</dt>
						<dd class={dd}>
							<PriorityIcon priority={t.priority} class="size-4 text-ink-400" />
							{priorityLabel(t.priority)}
						</dd>
					</div>
					<div>
						<dt class={dt}>Created</dt>
						<dd class={dd}>{dateTimeFmt.format(new Date(t.createdAt))}</dd>
					</div>
				</dl>
			{/if}
		</aside>
	</div>
</div>
