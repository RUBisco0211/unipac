<script setup lang="ts">
import TitleBar from '@/components/common/TitleBar.vue'
import { TooltipProvider } from '@/components/ui/tooltip'
import { useSidebar } from '@/composables/useSidebar'
import Page from '@/components/common/Page.vue'

const { isCollapsed, toggleSidebar } = useSidebar()
</script>

<template>
    <TooltipProvider>
        <div
            class="flex select-none h-screen flex-col overflow-hidden bg-[hsl(var(--background))] text-[hsl(var(--foreground))]"
        >
            <TitleBar type="system" :collapsed="isCollapsed" @toggle-sidebar="toggleSidebar" />

            <div class="grid min-h-0 flex-1 grid-cols-[auto_minmax(0,1fr)]">
                <Sidebar :is-collapsed="isCollapsed" />

                <Page>
                    <RouterView />
                </Page>
            </div>
        </div>
    </TooltipProvider>
</template>
