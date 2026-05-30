<script setup lang="ts">
import { Toaster } from 'vue-sonner'
import { useTheme } from '@/composables/useTheme'
import { useWailsEvent } from './composables/useWailsEvents';
import { useRouter } from 'vue-router';
import { onMounted } from 'vue';
import { GetCachedPackages } from '../wailsjs/go/main/App';
const router = useRouter()

useTheme()
useWailsEvent("open_settings", () => {
    router.push("settings")
})
onMounted(async () => {
    try {
        const res = await GetCachedPackages()
        console.log(res);
    } catch (error) {
        console.error("Failed to get installed packages:", error)
    } 
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
