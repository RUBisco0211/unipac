<script setup lang="ts">
import { Box } from 'lucide-vue-next'

const { type = 'system' } = defineProps<{
  type: 'system' | 'custom' | 'hidden'
}>()
</script>
<template>
  <div
    v-if="type === 'custom'"
    class="title-bar theme-panel-subtle theme-border h-12 backdrop-blur-md border-b flex items-center px-4 shrink-0 select-none"
  >
    <!-- Traffic Lights -->
    <div class="flex items-center space-x-2 w-60 shrink-0 group">
      <div
        class="w-4 h-4 rounded-full bg-[#ff5f56] border border-[#e0443e] shadow-sm cursor-pointer flex items-center justify-center text-black/60"
      >
        <svg
          class="w-2.5 h-2.5 opacity-0 group-hover:opacity-100 transition-opacity duration-75"
          viewBox="0 0 14 14"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
        >
          <path d="M3 3L11 11M11 3L3 11"></path>
        </svg>
      </div>
      <div
        class="w-4 h-4 rounded-full bg-[#ffbd2e] border border-[#dea123] shadow-sm cursor-pointer flex items-center justify-center text-black/60"
      >
        <svg
          class="w-2.5 h-2.5 opacity-0 group-hover:opacity-100 transition-opacity duration-75"
          viewBox="0 0 14 14"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
        >
          <path d="M2 7H12"></path>
        </svg>
      </div>
      <div
        class="w-4 h-4 rounded-full bg-[#27c93f] border border-[#1aab29] shadow-sm cursor-pointer flex items-center justify-center text-black/60"
      >
        <svg
          class="w-2.5 h-2.5 opacity-0 group-hover:opacity-100 transition-opacity duration-75"
          viewBox="0 0 14 14"
          fill="currentColor"
        >
          <path d="M1.5 1.5h4v1.5h-2.5v2.5h-1.5v-4zm11 11h-4v-1.5h2.5v-2.5h1.5v4z"></path>
        </svg>
      </div>
    </div>

    <!-- App Title -->
    <div class="theme-text-secondary flex-1 flex justify-center items-center text-sm font-semibold">
      <Box class="w-4 h-4 mr-2" />
      PackManager
    </div>

    <!-- Right Spacer for centering -->
    <div class="w-60 shrink-0"></div>
  </div>
  <div v-else-if="type === 'system'" class="title-bar"></div>
</template>
<style>
.title-bar {
  height: 32px;
}
</style>
