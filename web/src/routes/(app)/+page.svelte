<script lang="ts">
	import Icon, { type IconName } from '$lib/components/Icon.svelte';
	import TicketRow from '$lib/components/TicketRow.svelte';

	let { data } = $props();

	const isStaff = $derived(data.user?.role === 'admin' || data.user?.role === 'agent');
	const firstName = $derived(data.user?.name.split(' ')[0] ?? '');
	const greeting = (() => {
		const h = new Date().getHours();
		return h < 12 ? 'Good morning' : h < 18 ? 'Good afternoon' : 'Good evening';
	})();

	type Card = { label: string; value: number; icon: IconName; href: string };
	const cards = $derived<Card[]>(
		[
			{ label: 'Unassigned', value: data.stats.unassigned, icon: 'inbox', href: '/tickets?view=unassigned' },
			{ label: 'In progress', value: data.stats.inProgress, icon: 'ticket', href: '/tickets?view=in_progress' },
			...(isStaff
				? [{ label: 'Assigned to me', value: data.stats.assignedToMe, icon: 'user' as IconName, href: '/tickets?assignee=me' }]
				: []),
			{ label: 'Pending', value: data.stats.pending, icon: 'refresh', href: '/tickets?view=pending' },
			{ label: 'Resolved this week', value: data.stats.resolvedThisWeek, icon: 'check', href: '/tickets?view=resolved' }
		]
	);
</script>

<svelte:head>
	<title>Dashboard · Lithium</title>
</svelte:head>

<div class="mx-auto max-w-7xl px-4 py-10 sm:px-6">
	<h1 class="text-2xl font-semibold tracking-tight">{greeting}, {firstName}</h1>
	<p class="mt-1 text-sm text-ink-400">
		{isStaff ? "Here's what's happening in your queue." : "Here's where your requests are at."}
	</p>

	<div class={['mt-8 grid grid-cols-2 gap-3 sm:grid-cols-3', cards.length === 5 ? 'lg:grid-cols-5' : 'lg:grid-cols-4']}>
		{#each cards as card (card.label)}
			<a
				href={card.href}
				class="group rounded-xl border border-ink-800 bg-ink-900/60 p-4 transition hover:border-ink-700 hover:bg-ink-900 focus-visible:ring-2 focus-visible:ring-primary-400/40 focus-visible:outline-none"
			>
				<div class="flex items-center justify-between">
					<p class="text-[13px] text-ink-400">{card.label}</p>
					<Icon name={card.icon} class="size-4 text-ink-500 transition group-hover:text-primary-400" />
				</div>
				<p class="mt-3 text-2xl font-semibold tracking-tight text-ink-100 tabular-nums">{card.value.toLocaleString()}</p>
			</a>
		{/each}
	</div>

	<section class="mt-8 overflow-hidden rounded-xl border border-ink-800 bg-ink-900/60">
		<div class="flex items-center justify-between border-b border-ink-800 px-5 py-3.5">
			<h2 class="text-sm font-semibold">{isStaff ? 'Recent activity' : 'Your active tickets'}</h2>
			{#if data.recent.total > 0}
				<a href="/tickets" class="text-sm text-primary-300 hover:text-primary-200">
					View all{data.recent.total > data.recent.tickets.length ? ` ${data.recent.total.toLocaleString()}` : ''}
				</a>
			{/if}
		</div>
		{#if data.recent.tickets.length === 0}
			<div class="flex flex-col items-center px-6 py-16 text-center">
				<span class="flex size-11 items-center justify-center rounded-xl bg-primary-400/10 text-primary-400">
					<Icon name="inbox" class="size-5" />
				</span>
				<p class="mt-4 text-sm font-medium text-ink-200">No active tickets</p>
				<p class="mt-1 text-sm text-ink-500">
					{isStaff ? 'New and ongoing tickets will show up here.' : 'Need help with something?'}
				</p>
				<a href="/tickets/new" class="mt-4 text-sm font-medium text-primary-300 hover:text-primary-200">Raise a ticket</a>
			</div>
		{:else}
			<ul class="divide-y divide-ink-800 p-1.5">
				{#each data.recent.tickets as t (t.id)}
					<li><TicketRow ticket={t} showRequester={isStaff} /></li>
				{/each}
			</ul>
		{/if}
	</section>
</div>
