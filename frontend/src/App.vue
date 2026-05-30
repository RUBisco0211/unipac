<script setup lang="ts">
import { Toaster } from 'vue-sonner'
import { useTheme } from '@/composables/useTheme'
import { useWailsEvent } from './composables/useWailsEvents'
import { useRouter } from 'vue-router'
const router = useRouter()

useTheme()
useWailsEvent('open_settings', () => {
    void router.push('/settings')
})
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
