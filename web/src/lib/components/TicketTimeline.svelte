<script lang="ts">
	import type { Comment, Priority, TicketStatus } from '$lib/api';
	import { dateTimeFmt, timeAgo } from '$lib/format';
	import { priorityLabel } from '$lib/tickets';
	import Logo from './Logo.svelte';
	import Icon from './Icon.svelte';
	import { parseMentions } from '$lib/mentions';
	import { page } from '$app/state';
	import { ticketRef } from '$lib/tickets';
	import Avatar from './Avatar.svelte';
	import PriorityIcon from './PriorityIcon.svelte';
	import StatusBadge from './StatusBadge.svelte';

	// The conversation on a ticket: replies, with Lithium's automatic updates
	// about changes slotted in between.
	let { comments, requesterId }: { comments: Comment[]; requesterId: number } = $props();

	const isStaffRole = (role?: string) => role === 'admin' || role === 'agent';
	const actor = (c: Comment) => c.author?.name ?? 'Someone';
	const strong = 'font-medium text-ink-100';
	const prefix = $derived(page.data.settings?.ticketPrefix ?? 'TID');
</script>

{#snippet ticketLink(id: string)}
	<a href="/tickets/{id}" class="font-mono text-[13px] font-medium text-ink-100 underline-offset-2 hover:text-primary-300 hover:underline">
		{ticketRef(Number(id), prefix)}
	</a>
{/snippet}

{#snippet when(iso: string)}
	<time datetime={iso} title={dateTimeFmt.format(new Date(iso))} class="text-xs text-ink-500">{timeAgo(iso)}</time>
{/snippet}

<ol class="space-y-4">
	{#each comments as c (c.id)}
		{#if c.kind === 'comment'}
			<li class="flex gap-3">
				<Avatar
					name={c.author?.name ?? '?'}
					src={c.author?.avatarUrl}
					size={32}
					tone={c.author && c.author.id === requesterId ? 'neutral' : 'soft'}
					class="mt-0.5"
				/>
				<div class={['min-w-0 flex-1 rounded-xl border', c.internal ? 'tag-amber internal-note border-dashed' : 'border-ink-800 bg-ink-900/60']}>
					<div class="flex flex-wrap items-center gap-x-2 gap-y-0.5 border-b border-ink-800 px-4 py-2.5">
						<span class="text-sm font-medium text-ink-100">{c.author?.name ?? 'Deleted user'}</span>
						{#if isStaffRole(c.author?.role)}
							<span class="rounded-full border border-primary-400/30 bg-primary-400/10 px-1.5 py-px text-[10px] font-medium text-primary-200">
								Staff
							</span>
						{:else if c.author?.id === requesterId}
							<span class="rounded-full border border-ink-700 px-1.5 py-px text-[10px] font-medium text-ink-400">Requester</span>
						{/if}
						{#if c.internal}
							<span class="tag-chip inline-flex items-center gap-1 rounded-full px-1.5 py-px text-[10px] font-medium">
								<Icon name="lock" class="size-2.5" /> Internal note
							</span>
						{/if}
						<span class="ml-auto">{@render when(c.createdAt)}</span>
					</div>
					<div class="px-4 py-3 text-sm leading-relaxed whitespace-pre-wrap text-ink-200 break-words">{#each parseMentions(c.body ?? '') as seg, i (i)}{#if 'mention' in seg}<span class="rounded bg-primary-400/10 px-0.5 font-medium text-primary-300" title={seg.group ? 'Group' : undefined}>@{seg.mention}</span>{:else}{seg.text}{/if}{/each}</div>
				</div>
			</li>
		{:else}
			<!-- Lithium update -->
			<li class="flex items-start gap-3 pl-1.5">
				<span class="mt-0.5 shrink-0"><Logo size={20} /></span>
				<p class="flex min-w-0 flex-1 flex-wrap items-center gap-x-1.5 gap-y-1 pt-0.5 text-sm text-ink-400">
					<span class={strong}>Lithium</span>
					<span class="text-ink-500">·</span>
					{#if c.event === 'status'}
						<span><span class="text-ink-200">{actor(c)}</span> changed the status from</span>
						<StatusBadge status={c.from as TicketStatus} />
						<span>to</span>
						<StatusBadge status={c.to as TicketStatus} />
					{:else if c.event === 'priority'}
						<span><span class="text-ink-200">{actor(c)}</span> changed the priority from</span>
						<span class="inline-flex items-center gap-1 text-ink-200">
							<PriorityIcon priority={c.from as Priority} class="size-3.5" />{priorityLabel(c.from as Priority)}
						</span>
						<span>to</span>
						<span class="inline-flex items-center gap-1 text-ink-200">
							<PriorityIcon priority={c.to as Priority} class="size-3.5" />{priorityLabel(c.to as Priority)}
						</span>
					{:else if c.event === 'category'}
						<span class="text-ink-200">{actor(c)}</span>
						{#if !c.from}
							<span>set the category to <span class={strong}>{c.to}</span></span>
						{:else if !c.to}
							<span>removed the category <span class={strong}>{c.from}</span></span>
						{:else}
							<span>changed the category from <span class={strong}>{c.from}</span> to <span class={strong}>{c.to}</span></span>
						{/if}
					{:else if c.event === 'assignee'}
						<span class="text-ink-200">{actor(c)}</span>
						{#if !c.from}
							<span>
								{c.to === c.author?.name ? 'took this ticket' : 'assigned this to'}
								{#if c.to !== c.author?.name}<span class={strong}>{c.to}</span>{/if}
							</span>
						{:else if !c.to}
							<span>unassigned <span class={strong}>{c.from}</span></span>
						{:else}
							<span>reassigned this from <span class={strong}>{c.from}</span> to <span class={strong}>{c.to}</span></span>
						{/if}
					{:else if c.event === 'parent'}
						<span class="text-ink-200">{actor(c)}</span>
						{#if !c.from}
							<span>linked this under {@render ticketLink(c.to)}</span>
						{:else if !c.to}
							<span>unlinked this from {@render ticketLink(c.from)}</span>
						{:else}
							<span>moved this from {@render ticketLink(c.from)} to {@render ticketLink(c.to)}</span>
						{/if}
					{:else if c.event === 'child'}
						<span class="text-ink-200">{actor(c)}</span>
						{#if c.to}
							<span>linked {@render ticketLink(c.to)} under this ticket</span>
						{:else}
							<span>unlinked {@render ticketLink(c.from)} from this ticket</span>
						{/if}
					{/if}
					{#if c.internal}
						<span title="Only staff can see this" class="text-ink-500"><Icon name="lock" class="inline size-3" /></span>
					{/if}
					<span class="text-ink-500">·</span>
					{@render when(c.createdAt)}
				</p>
			</li>
		{/if}
	{/each}
</ol>
