<script lang="ts">
	import Icon from './Icon.svelte';

	// The red sensitive mode bar (Lithium settings → Appearance).
	let { message }: { message: string } = $props();
</script>

<div
	role="note"
	aria-label="Sensitive mode"
	class="flex items-center justify-center gap-2 bg-[#c53030] px-4 py-1.5 text-center text-[13px] font-semibold text-white"
>
	<Icon name="shieldAlert" class="size-4 shrink-0" />
	<span class="min-w-0">{message}</span>
</div>
