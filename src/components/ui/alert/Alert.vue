<script setup lang="ts">
import { cva } from 'class-variance-authority'
import { computed } from 'vue'
import { cn } from '@/lib/utils'

const alertVariants = cva(
    'relative w-full rounded-lg border px-4 py-3 text-sm grid gap-1 [&>svg~*]:pl-7 [&>svg]:absolute [&>svg]:left-4 [&>svg]:top-4 [&>svg]:text-current',
    {
        variants: {
            variant: {
                default: 'bg-[hsl(var(--card))] text-[hsl(var(--card-foreground))]',
                destructive:
                    'border-[hsl(var(--destructive)/0.5)] text-[hsl(var(--destructive))] [&>svg]:text-[hsl(var(--destructive))]',
            },
        },
        defaultVariants: {
            variant: 'default',
        },
    }
)

const props = withDefaults(
    defineProps<{
        class?: string
        variant?: 'default' | 'destructive'
    }>(),
    {
        variant: 'default',
    }
)

const classes = computed(() => cn(alertVariants({ variant: props.variant }), props.class))
</script>

<template>
    <div :class="classes" role="alert">
        <slot />
    </div>
</template>
