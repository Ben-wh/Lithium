<script lang="ts">
	// The red frame around every page while Lithium is in lockdown mode. The
	// page content is inset by --lockdown-inset (see the root layout), so the
	// frame never covers it.
	let { isAdmin = false }: { isAdmin?: boolean } = $props();

	const repeat = (n: number) => Array.from({ length: n }, () => 'LOCKDOWN MODE');
</script>

<!-- The frame is outside the content wrapper, so it sets the inset itself (keep in step with +layout.svelte). -->
<div
	class="lockdown-frame pointer-events-none fixed inset-0 z-[60]"
	style:--lockdown-inset="1.5rem"
	role="alert"
	aria-label="Lockdown mode is on"
>
	<!-- Top: the one with something to read and, for admins, a way out -->
	<div class="bar absolute inset-x-0 top-0 flex h-(--lockdown-inset) items-center justify-center gap-3 px-4">
		<span class="truncate">LOCKDOWN MODE · Signing in is turned off for everyone else</span>
		{#if isAdmin}
			<a
				href="/settings?section=lockdown"
				class="pointer-events-auto shrink-0 rounded bg-white/15 px-2 py-0.5 text-[11px] tracking-normal underline-offset-2 hover:bg-white/25"
			>
				Manage
			</a>
		{/if}
	</div>
	<div class="bar absolute inset-x-0 bottom-0 flex h-(--lockdown-inset) items-center justify-around overflow-hidden whitespace-nowrap">
		{#each repeat(6) as text, i (i)}<span>{text}</span>{/each}
	</div>
	<div class="bar side absolute top-(--lockdown-inset) bottom-(--lockdown-inset) left-0 flex w-(--lockdown-inset) items-center justify-around overflow-hidden">
		{#each repeat(4) as text, i (i)}<span>{text}</span>{/each}
	</div>
	<div class="bar side absolute top-(--lockdown-inset) right-0 bottom-(--lockdown-inset) flex w-(--lockdown-inset) items-center justify-around overflow-hidden">
		{#each repeat(4) as text, i (i)}<span>{text}</span>{/each}
	</div>
</div>

<style>
	.bar {
		background: repeating-linear-gradient(-45deg, #c53030 0 14px, #b02a2a 14px 28px);
		color: #fff;
		font-size: 11px;
		font-weight: 800;
		letter-spacing: 0.14em;
	}
	/* In vertical writing mode a flex row runs top to bottom. */
	.side {
		writing-mode: vertical-rl;
	}
	.bar span {
		white-space: nowrap;
		padding-inline: 1.5rem;
		flex-shrink: 0;
	}
	@media (prefers-reduced-motion: no-preference) {
		.lockdown-frame {
			animation: lockdown-pulse 2.4s ease-in-out infinite;
		}
	}
	@keyframes lockdown-pulse {
		50% {
			filter: brightness(1.15);
		}
	}
</style>
