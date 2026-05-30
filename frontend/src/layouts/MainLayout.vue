<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import TitleBar from '@/components/common/TitleBar.vue'
import { TooltipProvider } from '@/components/ui/tooltip'
import { useSidebar } from '@/composables/useSidebar'
import Page from '@/components/common/Page.vue'

const { t } = useI18n()

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
                    <RouterView v-slot="{ Component }">
                        <Transition name="slide-up" mode="out-in">
                            <KeepAlive>
                                <component :is="Component" :key="$route.fullPath" />
                            </KeepAlive>
                        </Transition>
                    </RouterView>
                </Page>
            </div>
        </div>
    </TooltipProvider>
</template>
<style>
.slide-up-enter-active {
    transition: all 0.25s ease;
}

.slide-up-enter-from {
    transform: translateY(20px);
    opacity: 0;
}

.slide-up-enter-to {
    transform: translateY(0);
    opacity: 1;
}
</style>
