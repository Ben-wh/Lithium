<script lang="ts">
	import Avatar from '$lib/components/Avatar.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import TicketRow from '$lib/components/TicketRow.svelte';
	import { formatDate, timeAgo } from '$lib/format';

	let { data } = $props();
	const p = $derived(data.person);
	const isAdmin = $derived(data.user?.role === 'admin');
	const roleLabels = { admin: 'Admin', agent: 'Agent', user: 'User' } as const;

	const stats = $derived([
		{ label: 'Open tickets', value: p.openTickets.toLocaleString() },
		{ label: 'All tickets', value: p.totalTickets.toLocaleString() },
		{ label: 'Last ticket', value: p.lastTicketAt ? timeAgo(p.lastTicketAt) : 'Never' },
		{ label: 'Member since', value: formatDate(p.createdAt) }
	]);

	const tabClass = (active: boolean) =>
		[
			'relative shrink-0 pb-3 text-sm font-medium transition',
			active
				? 'text-ink-100 after:absolute after:inset-x-0 after:bottom-0 after:h-0.5 after:rounded-full after:bg-primary-400'
				: 'text-ink-400 hover:text-ink-100'
		].join(' ');
</script>

<svelte:head>
	<title>{p.name} · User book · Lithium</title>
</svelte:head>

<div class="mx-auto max-w-5xl px-4 py-10 sm:px-6">
	<a href="/user-book" class="inline-flex items-center gap-1.5 text-sm text-ink-400 transition hover:text-ink-100">
		<Icon name="arrowLeft" class="size-4" /> User book
	</a>

	<div class="mt-4 flex flex-wrap items-center gap-5">
		<Avatar name={p.name} src={p.avatarUrl} size={72} tone="primary" />
		<div class="min-w-0 flex-1">
			<h1 class="flex flex-wrap items-center gap-2 text-2xl font-semibold tracking-tight">
				{p.name}
				<span class="rounded-full border border-ink-700 px-2 py-0.5 text-xs font-medium text-ink-400">{roleLabels[p.role]}</span>
			</h1>
			<a href="mailto:{p.email}" class="mt-0.5 inline-block text-sm text-ink-400 hover:text-primary-300">{p.email}</a>
		</div>
		<div class="flex flex-wrap gap-2">
			{#if isAdmin}
				<a
					href="/admin?tab=users"
					class="inline-flex h-9 items-center gap-1.5 rounded-lg border border-ink-700 px-3.5 text-sm text-ink-200 transition hover:border-primary-400/40 hover:text-primary-200"
				>
					<Icon name="shield" class="size-4" /> Manage account
				</a>
			{/if}
			<a
				href="/tickets/new?requester={p.id}"
				class="inline-flex h-9 items-center gap-1.5 rounded-lg bg-primary-400 px-3.5 text-sm font-semibold text-primary-950 transition hover:bg-primary-300"
			>
				<Icon name="plus" class="size-4" /> Raise a ticket for them
			</a>
		</div>
	</div>

	<dl class="mt-8 grid grid-cols-2 gap-3 lg:grid-cols-4">
		{#each stats as s (s.label)}
			<div class="rounded-xl border border-ink-800 bg-ink-900/60 p-4">
				<dt class="text-[13px] text-ink-400">{s.label}</dt>
				<dd class="mt-2 text-xl font-semibold tracking-tight text-ink-100 tabular-nums">{s.value}</dd>
			</div>
		{/each}
	</dl>

	<section class="mt-8" aria-labelledby="history-heading">
		<h2 id="history-heading" class="text-sm font-semibold">Ticket history</h2>
		<nav aria-label="Ticket status" class="tab-row-line mt-3 flex gap-5">
			<a href="?" data-sveltekit-replacestate data-sveltekit-noscroll class={tabClass(data.view === 'active')}>Active</a>
			<a href="?view=all" data-sveltekit-replacestate data-sveltekit-noscroll class={tabClass(data.view === 'all')}>All</a>
		</nav>

		<div class="mt-4 rounded-xl border border-ink-800 bg-ink-900/60">
			{#if data.history.tickets.length === 0}
				<p class="px-6 py-12 text-center text-sm text-ink-500">
					{data.view === 'active' ? 'No active tickets.' : 'No tickets yet.'}
					{#if data.view === 'active' && p.totalTickets > 0}<a href="?view=all" class="text-primary-300 hover:text-primary-200">See all</a>{/if}
				</p>
			{:else}
				<ul class="divide-y divide-ink-800 p-1.5">
					{#each data.history.tickets as t (t.id)}
						<li><TicketRow ticket={t} /></li>
					{/each}
				</ul>
			{/if}
		</div>
		{#if data.history.total > data.history.tickets.length}
			<p class="mt-3 text-center text-xs text-ink-500">
				Showing the {data.history.tickets.length} most recent of {data.history.total.toLocaleString()}.
				<a href="/tickets?view=all&q={encodeURIComponent(p.email)}" class="text-primary-300 hover:text-primary-200">See all in Tickets</a>
			</p>
		{/if}
	</section>
</div>
