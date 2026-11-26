<script lang="ts">
	import { onMount } from 'svelte';
	import { afterNavigate, goto } from '$app/navigation';
	import { page } from '$app/state';
	import { notificationsApi, type Notification } from '$lib/api';
	import { timeAgo } from '$lib/format';
	import { notifications } from '$lib/notifications.svelte';
	import { ticketRef } from '$lib/tickets';
	import Avatar from './Avatar.svelte';
	import Icon from './Icon.svelte';
	import Logo from './Logo.svelte';

	let open = $state(false);
	let items = $state<Notification[]>([]);
	let loading = $state(false);
	let loadFailed = $state(false);
	let rootEl: HTMLElement | undefined = $state();

	const prefix = $derived(page.data.settings?.ticketPrefix ?? 'TID');
	const badge = $derived(notifications.unread > 99 ? '99+' : String(notifications.unread));

	// Keep the count fresh: on load, every 30s, when the tab regains focus,
	// and as you move around the app.
	onMount(() => {
		notifications.refresh();
		const timer = setInterval(() => notifications.refresh(), 30_000);
		const onVisible = () => document.visibilityState === 'visible' && notifications.refresh();
		document.addEventListener('visibilitychange', onVisible);
		return () => {
			clearInterval(timer);
			document.removeEventListener('visibilitychange', onVisible);
		};
	});
	afterNavigate(() => {
		open = false;
		notifications.refresh();
	});

	async function toggle() {
		open = !open;
		if (!open) return;
		loading = true;
		loadFailed = false;
		try {
			const res = await notificationsApi.list();
			items = res.items;
			notifications.unread = res.unread;
		} catch {
			loadFailed = true;
		} finally {
			loading = false;
		}
	}

	async function openItem(n: Notification) {
		if (!n.read) {
			n.read = true;
			notificationsApi.markRead({ ids: [n.id] }).then((r) => (notifications.unread = r.unread));
		}
		open = false;
		await goto(`/tickets/${n.ticket.id}`);
	}

	async function markAllRead() {
		items = items.map((n) => ({ ...n, read: true }));
		notifications.unread = (await notificationsApi.markRead()).unread;
	}

	function onWindowClick(e: MouseEvent) {
		if (open && rootEl && !rootEl.contains(e.target as Node)) open = false;
	}
	function onKeydown(e: KeyboardEvent) {
		if (open && e.key === 'Escape') open = false;
	}
</script>

<svelte:window onclick={onWindowClick} onkeydown={onKeydown} />

<div class="relative" bind:this={rootEl}>
	<button
		type="button"
		onclick={toggle}
		aria-haspopup="true"
		aria-expanded={open}
		aria-label={notifications.unread ? `Notifications, ${notifications.unread} unread` : 'Notifications'}
		class="relative flex size-8 items-center justify-center rounded-md text-ink-400 transition hover:bg-ink-900 hover:text-ink-100 focus-visible:ring-2 focus-visible:ring-primary-400/40 focus-visible:outline-none"
	>
		<Icon name="bell" class="size-[18px]" />
		{#if notifications.unread > 0}
			<span
				class="absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-primary-400 px-1 text-[10px] font-bold text-primary-950 ring-2 ring-ink-950"
			>
				{badge}
			</span>
		{/if}
	</button>

	{#if open}
		<div
			class="absolute right-0 mt-2 w-[min(24rem,calc(100vw-2rem))] origin-top-right overflow-hidden rounded-xl border border-ink-800 bg-ink-900 shadow-2xl shadow-shade/50"
		>
			<div class="flex items-center justify-between border-b border-ink-800 px-4 py-3">
				<h2 class="text-sm font-semibold">Notifications</h2>
				{#if items.some((n) => !n.read)}
					<button type="button" onclick={markAllRead} class="text-xs font-medium text-primary-300 hover:text-primary-200">
						Mark all as read
					</button>
				{/if}
			</div>

			<div class="max-h-[28rem] overflow-y-auto">
				{#if loading && items.length === 0}
					<p class="px-4 py-10 text-center text-sm text-ink-500">Loading…</p>
				{:else if loadFailed}
					<p class="px-4 py-10 text-center text-sm text-danger-300">Couldn't load notifications.</p>
				{:else if items.length === 0}
					<div class="flex flex-col items-center px-4 py-10 text-center">
						<span class="flex size-10 items-center justify-center rounded-xl bg-primary-400/10 text-primary-400">
							<Icon name="bell" class="size-5" />
						</span>
						<p class="mt-3 text-sm font-medium text-ink-200">You're all caught up</p>
						<p class="mt-0.5 text-xs text-ink-500">Replies and updates on your tickets will show here.</p>
					</div>
				{:else}
					<ul class="divide-y divide-ink-800">
						{#each items as n (n.id)}
							<li>
								<button
									type="button"
									onclick={() => openItem(n)}
									class={[
										'flex w-full items-start gap-3 px-4 py-3 text-left transition hover:bg-ink-850 focus-visible:bg-ink-850 focus-visible:outline-none',
										!n.read && 'bg-primary-400/5'
									]}
								>
									{#if n.actor}
										<Avatar name={n.actor.name} src={n.actor.avatarUrl} size={32} />
									{:else}
										<span class="shrink-0"><Logo size={32} /></span>
									{/if}
									<span class="min-w-0 flex-1">
										<span class="block text-sm text-ink-200">
											{#if n.kind === 'sla'}
												<span class="font-medium text-ink-100">SLA:</span>
												{n.summary}
											{:else}
												<span class="font-medium text-ink-100">{n.actor?.name ?? 'Someone'}</span>
												{n.summary}
											{/if}
										</span>
										<span class="mt-0.5 block truncate text-xs text-ink-500">
											<span class="font-mono">{ticketRef(n.ticket.id, prefix)}</span> · {n.ticket.subject}
										</span>
										<span class="mt-0.5 block text-[11px] text-ink-500">{timeAgo(n.createdAt)}</span>
									</span>
									{#if !n.read}
										<span class="mt-1.5 size-2 shrink-0 rounded-full bg-primary-400" aria-label="Unread"></span>
									{/if}
								</button>
							</li>
						{/each}
					</ul>
				{/if}
			</div>
		</div>
	{/if}
</div>
