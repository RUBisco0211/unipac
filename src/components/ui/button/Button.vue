<script setup lang="ts">
import { cva } from 'class-variance-authority'
import { computed } from 'vue'
import { cn } from '@/lib/utils'

const buttonVariants = cva(
    'select-none inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[hsl(var(--ring))] focus-visible:ring-offset-2 focus-visible:ring-offset-[hsl(var(--background))] disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:size-4 shrink-0',
    {
        variants: {
            variant: {
                default:
                    'bg-[hsl(var(--primary))] text-[hsl(var(--primary-foreground))] shadow-sm hover:bg-[hsl(var(--primary)/0.92)]',
                destructive:
                    'bg-[hsl(var(--destructive))] text-white shadow-sm hover:bg-[hsl(var(--destructive)/0.9)]',
                outline:
                    'border border-[hsl(var(--border))] bg-[hsl(var(--background))] shadow-xs hover:bg-[hsl(var(--accent))] hover:text-[hsl(var(--accent-foreground))]',
                secondary:
                    'bg-[hsl(var(--secondary))] text-[hsl(var(--secondary-foreground))] shadow-xs hover:bg-[hsl(var(--secondary)/0.8)]',
                ghost: 'hover:bg-[hsl(var(--accent))] hover:text-[hsl(var(--accent-foreground))]',
                link: 'text-[hsl(var(--primary))] underline-offset-4 hover:underline',
            },
            size: {
                default: 'h-9 px-4 py-2',
                sm: 'h-8 rounded-md px-3 text-xs',
                lg: 'h-10 rounded-md px-8',
                icon: 'size-9',
            },
        },
        defaultVariants: {
            variant: 'default',
            size: 'default',
        },
    }
)

type Props = {
    variant?: 'default' | 'destructive' | 'outline' | 'secondary' | 'ghost' | 'link'
    size?: 'default' | 'sm' | 'lg' | 'icon'
    class?: string
    type?: 'button' | 'submit' | 'reset'
    disabled?: boolean
}

const props = withDefaults(defineProps<Props>(), {
    variant: 'default',
    size: 'default',
    type: 'button',
})

const classes = computed(() =>
    cn(buttonVariants({ variant: props.variant, size: props.size }), props.class)
)
</script>

<template>
    <button :type="props.type" :disabled="props.disabled" :class="classes">
        <slot />
    </button>
</template>
