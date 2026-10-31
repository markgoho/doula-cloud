<script lang="ts">
	import { resolve } from '$app/paths';
</script>

<svelte:head><title>You have signed out – DoulaCloud operations</title></svelte:head>

<h1>You have signed out</h1>
<p><a href={resolve('/sign-in')}>Sign in again</a></p>
