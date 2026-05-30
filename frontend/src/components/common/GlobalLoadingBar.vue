<script setup lang="ts">
import { computed } from 'vue'
import { useLoadingBar } from '@/composables/useLoadingBar'

const { isLoading } = useLoadingBar()
const wrapperClass = computed(() => (isLoading.value ? 'opacity-100' : 'opacity-0'))
</script>

<template>
    <div
        class="pointer-events-none fixed inset-x-0 top-9 z-[70] h-[2px] transition-opacity duration-200"
        :class="wrapperClass"
    >
        <div class="loading-bar-track h-full w-full overflow-hidden bg-[hsl(var(--border)/0.35)]">
            <div class="loading-bar-indicator h-full w-[28%] bg-[hsl(var(--primary))]" />
        </div>
    </div>
</template>

<style scoped>
.loading-bar-indicator {
    animation: loading-bar-loop 1.15s ease-in-out infinite;
    box-shadow: 0 0 10px hsl(var(--primary) / 0.35);
}

@keyframes loading-bar-loop {
    0% {
        transform: translateX(-120%);
    }

    50% {
        transform: translateX(150%);
    }

    100% {
        transform: translateX(360%);
    }
}
</style>
