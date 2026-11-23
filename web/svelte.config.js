import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
export default {
	preprocess: vitePreprocess(),
	kit: {
		// Single-page app: Go serves index.html for every non-asset route.
		adapter: adapter({ pages: 'build', assets: 'build', fallback: 'index.html' })
	}
};
