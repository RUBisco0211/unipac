<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps<{
    class?: HTMLAttributes['class']
}>()
</script>

<template>
    <div
        :class="
            cn(
                'flex flex-col space-y-1.5 px-4 py-3 border-b border-[hsl(var(--border))]',
                props.class
            )
        "
    >
        <slot />
    </div>
</template>
