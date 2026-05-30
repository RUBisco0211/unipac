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
        position="top-right"
        theme="system"
        rich-colors
        close-button
        :offset="48"
        :toast-options="{ class: 'unipac-toast', descriptionClass: 'unipac-toast-description' }"
    />
    <router-view />
</template>
