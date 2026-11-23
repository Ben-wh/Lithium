<script lang="ts">
	import type { TicketLink, TicketStatus } from '$lib/api';
	import Avatar from '$lib/components/Avatar.svelte';
	import CategoryTag from '$lib/components/CategoryTag.svelte';
	import Icon from '$lib/components/Icon.svelte';
	import PriorityIcon from '$lib/components/PriorityIcon.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import { statusLabels, ticketRef } from '$lib/tickets';

	let { data } = $props();
	const prefix = $derived(data.settings.ticketPrefix);

	// Card size and spacing (px).
	const W = 232;
	const H = 104;
	const GAP_X = 24;
	const GAP_Y = 56;
	const PAD = 24;

	type Node = TicketLink & { kids: Node[]; x: number; y: number };

	// A tidy top-down tree: each leaf gets its own column, and each parent is
	// centred over its children.
	const layout = $derived.by(() => {
		const byId = new Map<number, Node>();
		for (const n of data.tree.nodes) byId.set(n.id, { ...n, kids: [], x: 0, y: 0 });
		const root = byId.get(data.tree.rootId)!;
		for (const n of byId.values()) {
			if (n.id !== root.id && n.parentId) byId.get(n.parentId)?.kids.push(n);
		}

		let column = 0;
		let maxDepth = 0;
		const place = (n: Node, depth: number) => {
			maxDepth = Math.max(maxDepth, depth);
			n.y = PAD + depth * (H + GAP_Y);
			if (n.kids.length === 0) {
				n.x = PAD + column++ * (W + GAP_X);
			} else {
				n.kids.forEach((k) => place(k, depth + 1));
				n.x = (n.kids[0].x + n.kids[n.kids.length - 1].x) / 2;
			}
		};
		place(root, 0);

		const nodes = [...byId.values()];
		const edges = nodes.flatMap((p) =>
			p.kids.map((k) => {
				const x1 = p.x + W / 2;
				const y1 = p.y + H;
				const x2 = k.x + W / 2;
				const y2 = k.y;
				const mid = y1 + GAP_Y / 2;
				return {
					id: `${p.id}-${k.id}`,
					d: `M${x1} ${y1} V${mid} H${x2} V${y2}`,
					fromRoot: p.id === root.id,
					x1,
					y1,
					x2,
					y2
				};
			})
		);
		return {
			root,
			nodes,
			edges,
			width: PAD * 2 + column * (W + GAP_X) - GAP_X,
			height: PAD * 2 + (maxDepth + 1) * (H + GAP_Y) - GAP_Y
		};
	});

	// Status summary for the header.
	const counts = $derived.by(() => {
		const c = new Map<TicketStatus, number>();
		for (const n of data.tree.nodes) if (!n.restricted) c.set(n.status, (c.get(n.status) ?? 0) + 1);
		return (Object.keys(statusLabels) as TicketStatus[]).filter((s) => c.has(s)).map((s) => ({ status: s, n: c.get(s)! }));
	});
	const open = $derived(data.tree.nodes.filter((n) => !n.restricted && n.status !== 'resolved' && n.status !== 'closed').length);
</script>

<svelte:head>
	<title>Incident {ticketRef(layout.root.id, prefix)} · Lithium</title>
</svelte:head>

<div class="mx-auto max-w-7xl px-4 py-10 sm:px-6">
	<a href="/tickets/{layout.root.id}" class="inline-flex items-center gap-1.5 text-sm text-ink-400 transition hover:text-ink-100">
		<Icon name="arrowLeft" class="size-4" /> {ticketRef(layout.root.id, prefix)}
	</a>

	<div class="mt-3 flex flex-wrap items-end justify-between gap-4">
		<div class="min-w-0">
			<p class="flex items-center gap-2 text-sm text-primary-300"><Icon name="tree" class="size-4" /> Incident tree</p>
			<h1 class="mt-1 text-2xl font-semibold tracking-tight break-words">{layout.root.subject}</h1>
			<p class="mt-1 text-sm text-ink-400">
				{data.tree.nodes.length} linked {data.tree.nodes.length === 1 ? 'ticket' : 'tickets'} · {open} still open.
				Click a ticket to open it in a new tab.
			</p>
		</div>
		<div class="flex flex-wrap gap-2">
			{#each counts as c (c.status)}
				<span class="flex items-center gap-1.5 text-xs text-ink-400"><StatusBadge status={c.status} /> {c.n}</span>
			{/each}
		</div>
	</div>

	{#if data.tree.truncated}
		<p class="mt-4 rounded-lg border border-ink-700 bg-ink-900 px-3 py-2 text-sm text-ink-200">
			This incident is very large, so only the first {data.tree.nodes.length} tickets are shown.
		</p>
	{/if}

	<!-- Diagram: scrolls in both directions if it's bigger than the screen. -->
	<div class="mt-6 overflow-auto rounded-xl border border-ink-800 bg-ink-900/40 bg-[radial-gradient(circle_at_center,var(--dot-grid)_1px,transparent_1px)] bg-size-[20px_20px]">
		<div class="relative mx-auto" style:width="{layout.width}px" style:height="{layout.height}px">

			<ul aria-label="Tickets in this incident">
				{#each layout.nodes as n (n.id)}
					{@const isRoot = n.id === layout.root.id}
					{@const done = n.status === 'resolved' || n.status === 'closed'}
					<li class="absolute" style:left="{n.x}px" style:top="{n.y}px" style:width="{W}px" style:height="{H}px">
						{#if n.restricted}
							<div
								title="Outside your groups"
								class="flex h-full flex-col items-center justify-center gap-1.5 rounded-xl border border-dashed border-ink-700 bg-ink-900/60 text-center"
							>
								<Icon name="lock" class="size-4 text-ink-500" />
								<span class="font-mono text-xs text-ink-500">{ticketRef(n.id, prefix)}</span>
								<span class="text-xs text-ink-500">Restricted ticket</span>
							</div>
						{:else}
						<a
							href="/tickets/{n.id}"
							target="_blank"
							rel="noopener"
							title="Open {ticketRef(n.id, prefix)} in a new tab"
							style:--ring-width={isRoot ? '1px' : '0px'}
							class={[
								'group flex h-full flex-col justify-between rounded-xl border bg-ink-900 p-3 shadow-lg shadow-shade/20 transition hover:-translate-y-0.5 hover:shadow-shade/40',
								'focus-visible:ring-2 focus-visible:ring-primary-400/60 focus-visible:outline-none',
								isRoot || n.priority === 'urgent' ? 'urgent-card' : 'border-ink-700 hover:border-ink-500',
								done && 'opacity-60 hover:opacity-100'
							]}
						>
							<div class="flex items-center gap-2">
								<PriorityIcon priority={n.priority} class="size-4 shrink-0 text-ink-400" />
								<span class="font-mono text-xs text-ink-500">{ticketRef(n.id, prefix)}</span>
								{#if isRoot}
									<span class="rounded-full bg-primary-400 px-1.5 py-px text-[10px] font-bold text-primary-950">Incident</span>
								{/if}
								<Icon name="external" class="ml-auto size-3.5 text-ink-500 opacity-0 transition group-hover:opacity-100" />
							</div>
							<p class="line-clamp-2 text-[13px] leading-snug font-medium text-ink-100">{n.subject}</p>
							<div class="flex items-center gap-2">
								<StatusBadge status={n.status} />
								{#if n.category}<CategoryTag category={n.category} class="min-w-0" />{/if}
								<span class="ml-auto">
									{#if n.assignee}
										<span title="Assigned to {n.assignee.name}"><Avatar name={n.assignee.name} src={n.assignee.avatarUrl} size={22} /></span>
									{:else}
										<span title="Unassigned" class="block size-[22px] rounded-full border border-dashed border-ink-500"></span>
									{/if}
								</span>
							</div>
						</a>
						{/if}
					</li>
				{/each}
			</ul>

			<svg class="pointer-events-none absolute inset-0" width={layout.width} height={layout.height} aria-hidden="true">
				<!-- Lines from the incident (root) ticket are drawn in the accent colour. -->
				{#each layout.edges as e (e.id)}
					<path
						d={e.d}
						fill="none"
						style:stroke={e.fromRoot ? 'var(--color-primary-300)' : 'var(--color-ink-500)'}
						stroke-width="2"
						stroke-linejoin="round"
						stroke-linecap="round"
					/>
					<circle cx={e.x1} cy={e.y1} r="3.5" style:fill={e.fromRoot ? 'var(--color-primary-300)' : 'var(--color-ink-500)'} />
					<circle cx={e.x2} cy={e.y2} r="3.5" style:fill={e.fromRoot ? 'var(--color-primary-300)' : 'var(--color-ink-500)'} />
				{/each}
			</svg>
		</div>
	</div>
</div>
