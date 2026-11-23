<script lang="ts">
	import type { Snippet } from 'svelte';

	// A switch with a sliding knob that shows ✓ when on and ✗ when off.
	// Still a real checkbox underneath (role="switch"), so keyboard and
	// screen readers work as normal.
	let {
		checked = $bindable(false),
		disabled = false,
		onchange,
		children,
		description
	}: {
		checked?: boolean;
		disabled?: boolean;
		onchange?: (checked: boolean) => void;
		children: Snippet;
		description?: Snippet;
	} = $props();
</script>

<label class={['flex items-start justify-between gap-6', disabled ? 'cursor-not-allowed opacity-60' : 'cursor-pointer']}>
	<span class="min-w-0">
		<span class="block text-sm font-medium text-ink-100">{@render children()}</span>
		{#if description}
			<span class="mt-0.5 block text-sm text-ink-400">{@render description()}</span>
		{/if}
	</span>

	<span class="relative mt-0.5 inline-flex h-6 w-11 shrink-0">
		<input
			type="checkbox"
			role="switch"
			bind:checked
			{disabled}
			onchange={() => onchange?.(checked)}
			class="peer sr-only"
		/>
		<!-- Track -->
		<span
			class="absolute inset-0 rounded-full bg-ink-500 transition-colors duration-200 peer-checked:bg-primary-400 peer-focus-visible:ring-3 peer-focus-visible:ring-primary-400/35"
		></span>
		<!-- Knob -->
		<span
			class="absolute top-0.5 left-0.5 flex size-5 items-center justify-center rounded-full bg-white shadow-sm shadow-black/30 transition-transform duration-200 ease-out peer-checked:translate-x-5"
			aria-hidden="true"
		>
			{#if checked}
				<svg class="size-3 text-primary-600" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<path d="M2.5 6.25 5 8.75l4.5-5" />
				</svg>
			{:else}
				<svg class="size-3 text-ink-500" viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round">
					<path d="M3.5 3.5l5 5m0-5-5 5" />
				</svg>
			{/if}
		</span>
	</span>
</label>
