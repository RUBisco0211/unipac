<script setup lang="ts">
import { cn } from '@/lib/utils'
import { computed } from 'vue'

const props = defineProps<{
    variant?: 'default' | 'destructive' | 'outline' | 'secondary' | 'ghost' | 'link'
    size?: 'default' | 'sm' | 'lg' | 'icon'
    class?: any
    disabled?: boolean
    type?: 'button' | 'submit' | 'reset'
}>()

const variants = {
    default:
        'bg-[var(--color-button-primary-bg)] text-[var(--color-button-primary-text)] hover:bg-[var(--color-button-primary-bg-hover)]',
    destructive:
        'bg-[var(--color-button-destructive-bg)] text-[var(--color-button-destructive-text)] hover:bg-[var(--color-button-destructive-bg-hover)]',
    outline:
        'border theme-border bg-[var(--color-button-outline-bg)] text-[var(--color-button-outline-text)] hover:bg-[var(--color-button-outline-bg-hover)]',
    secondary:
        'bg-[var(--color-button-secondary-bg)] text-[var(--color-button-secondary-text)] hover:bg-[var(--color-button-secondary-bg-hover)]',
    ghost: 'hover:bg-[var(--color-bg-subtle)] hover:text-[var(--color-text)]',
    link: 'text-[var(--color-text)] underline-offset-4 hover:underline',
}

const sizes = {
    default: 'h-9 px-4 py-2',
    sm: 'h-8 rounded-md px-3 text-xs',
    lg: 'h-10 rounded-md px-8',
    icon: 'h-9 w-9',
}

const computedClass = computed(() => {
    return cn(
        'inline-flex items-center justify-center whitespace-nowrap rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-[var(--color-focus)] disabled:pointer-events-none disabled:opacity-50',
        variants[props.variant || 'default'],
        sizes[props.size || 'default'],
        props.class
    )
})
</script>

<template>
    <button :type="type || 'button'" :disabled="disabled" :class="computedClass">
        <slot />
    </button>
</template>
