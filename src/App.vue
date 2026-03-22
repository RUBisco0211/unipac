<script setup lang="ts">
import { Toaster } from 'vue-sonner'
import GlobalLoadingBar from '@/components/common/GlobalLoadingBar.vue'
import { useTheme } from '@/composables/useTheme'

useTheme()
</script>

<template>
    <GlobalLoadingBar />
    <Toaster
        position="bottom-right"
        theme="system"
        rich-colors
        close-button
        :toast-options="{ class: 'text-[13px]' }"
    />
    <router-view />
</template>
