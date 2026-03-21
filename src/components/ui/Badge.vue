<script setup lang="ts">
import { cn } from '@/lib/utils'
import { computed } from 'vue'

const props = defineProps<{
    variant?: 'default' | 'secondary' | 'destructive' | 'outline'
    class?: any
}>()

const variants = {
    default:
        'border-transparent bg-[var(--color-button-primary-bg)] text-[var(--color-button-primary-text)]',
    secondary:
        'border-transparent bg-[var(--color-button-secondary-bg)] text-[var(--color-button-secondary-text)]',
    destructive:
        'border-transparent bg-[var(--color-button-destructive-bg)] text-[var(--color-button-destructive-text)]',
    outline: 'theme-text',
}

const computedClass = computed(() => {
    return cn(
        'inline-flex items-center rounded-full border theme-border px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-[var(--color-focus)] focus:ring-offset-2',
        variants[props.variant || 'default'],
        props.class
    )
})
</script>

<template>
    <div :class="computedClass">
        <slot />
    </div>
</template>
