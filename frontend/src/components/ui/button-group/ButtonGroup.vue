<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const props = withDefaults(
    defineProps<{
        class?: HTMLAttributes['class']
    }>(),
    {
        class: '',
    }
)
</script>

<template>
    <div :class="cn('inline-flex', props.class)">
        <slot />
    </div>
</template>
