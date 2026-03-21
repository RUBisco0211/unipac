<script setup lang="ts">
import { cn } from '@/lib/utils'

const props = defineProps<{
    modelValue?: string | number
    class?: any
    placeholder?: string
    type?: string
}>()

const emit = defineEmits(['update:modelValue'])
</script>

<template>
    <input
        :type="type || 'text'"
        :value="modelValue"
        :placeholder="placeholder"
        @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
        :class="
            cn(
                'flex h-9 w-full rounded-md border theme-border bg-transparent px-3 py-1 text-sm text-[var(--color-text)] shadow-sm transition-colors file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-[var(--color-text-muted)] focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-[var(--color-focus)] disabled:cursor-not-allowed disabled:opacity-50',
                props.class
            )
        "
    />
</template>
