<script lang="ts">
	import SensitiveBanner from '$lib/components/SensitiveBanner.svelte';
	import Navbar from '$lib/components/Navbar.svelte';

	let { data, children } = $props();
</script>

<!-- The root layout redirects to /login when signed out, so user is always set here. -->
{#if data.user}
	<!-- Workspace background chosen by an admin (Lithium settings → Appearance). -->
	<div class={['min-h-dvh', `app-bg-${data.settings.background}`, data.settings.background !== 'waves' && 'bg-fixed']}>
		<!-- The banner and navbar stay pinned together so the banner can't scroll away. -->
		<div class="sticky top-0 z-40">
			{#if data.settings.sensitiveMode}
				<SensitiveBanner message={data.settings.sensitiveMessage} />
			{/if}
			<Navbar user={data.user} />
		</div>
		<main>
			{@render children()}
		</main>
	</div>
{/if}
