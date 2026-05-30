<script setup lang="ts">
import { cn } from '@/lib/utils'

const props = defineProps<{
    modelValue?: string | number
    class?: string
    placeholder?: string
    type?: string
    disabled?: boolean
}>()

const emit = defineEmits<{
    'update:modelValue': [value: string]
}>()
</script>

<template>
    <input
        :type="type || 'text'"
        :value="modelValue"
        :placeholder="placeholder"
        :disabled="disabled"
        @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
        :class="
            cn(
                'flex h-9 w-full rounded-md border border-[hsl(var(--input))] bg-transparent px-3 py-1 text-sm shadow-xs transition-colors outline-none file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-[hsl(var(--muted-foreground))] focus-visible:border-[hsl(var(--ring))] focus-visible:ring-[3px] focus-visible:ring-[hsl(var(--ring)/0.2)] disabled:cursor-not-allowed disabled:opacity-50',
                props.class
            )
        "
    />
</template>
