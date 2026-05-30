<script setup lang="ts">
import { cva } from 'class-variance-authority'
import { computed } from 'vue'
import { cn } from '@/lib/utils'

const badgeVariants = cva(
    'inline-flex items-center rounded-md border px-2 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-[hsl(var(--ring))] focus:ring-offset-2',
    {
        variants: {
            variant: {
                default:
                    'border-transparent bg-[hsl(var(--primary))] text-[hsl(var(--primary-foreground))]',
                secondary:
                    'border-transparent bg-[hsl(var(--secondary))] text-[hsl(var(--secondary-foreground))]',
                destructive: 'border-transparent bg-[hsl(var(--destructive))] text-white',
                outline: 'border-[hsl(var(--border))] text-[hsl(var(--foreground))]',
            },
        },
        defaultVariants: {
            variant: 'default',
        },
    }
)

type Props = {
    variant?: 'default' | 'secondary' | 'destructive' | 'outline'
    class?: string
}

const props = withDefaults(defineProps<Props>(), {
    variant: 'default',
})

const classes = computed(() => cn(badgeVariants({ variant: props.variant }), props.class))
</script>

<template>
    <div :class="classes">
        <slot />
    </div>
</template>
