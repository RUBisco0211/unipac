<template>
    <tr
        class="border-b border-[hsl(var(--border)/0.45)] transition-colors hover:bg-[hsl(var(--muted)/0.3)] data-[state=selected]:bg-[hsl(var(--muted)/0.55)]"
    >
        <slot />
    </tr>
</template>
