<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import TitleBar from '@/components/common/TitleBar.vue'
import { TooltipProvider } from '@/components/ui/tooltip'

const AUTO_COLLAPSE_WIDTH = 1080
const AUTO_EXPAND_WIDTH = 1240

const router = useRouter()
const route = router.currentRoute
const isCollapsed = ref(false)
const { t } = useI18n()

const topNavItems = computed(
    () => route.value.matched[0]?.children.filter(i => i.meta?.position === 'top') ?? []
)
const bottomNavItems = computed(
    () => route.value.matched[0]?.children.filter(i => i.meta?.position === 'bottom') ?? []
)
function syncSidebarWithWidth() {
    const width = window.innerWidth

    if (width <= AUTO_COLLAPSE_WIDTH) {
        isCollapsed.value = true
        return
    }

    if (width >= AUTO_EXPAND_WIDTH) {
        isCollapsed.value = false
    }
}

function toggleSidebar() {
    isCollapsed.value = !isCollapsed.value
}

onMounted(() => {
    syncSidebarWithWidth()
    window.addEventListener('resize', syncSidebarWithWidth)
})

onBeforeUnmount(() => {
    window.removeEventListener('resize', syncSidebarWithWidth)
})
</script>

<template>
    <TooltipProvider>
        <div
            class="flex select-none h-screen flex-col overflow-hidden bg-[hsl(var(--background))] text-[hsl(var(--foreground))]"
        >
            <TitleBar type="system" :collapsed="isCollapsed" @toggle-sidebar="toggleSidebar" />

            <div class="grid min-h-0 flex-1 grid-cols-[auto_minmax(0,1fr)]">
                <aside
                    class="flex min-h-0 flex-col border-r border-[hsl(var(--border))] bg-[hsl(var(--sidebar))] transition-[width] duration-200"
                    :class="isCollapsed ? 'w-13' : 'w-48'"
                >
                    <nav class="flex flex-1 flex-col px-2 py-2">
                        <router-link
                            v-for="item in topNavItems"
                            :key="item.path"
                            :to="item.path"
                            class="mb-1 flex h-8 items-center rounded-md px-2.5 text-[13px] transition-colors"
                            :class="
                                route.path.startsWith(`/${item.path}`)
                                    ? 'bg-[hsl(var(--sidebar-accent))] font-medium text-[hsl(var(--sidebar-accent-foreground))]'
                                    : 'text-[hsl(var(--muted-foreground))] hover:bg-[hsl(var(--accent)/0.72)] hover:text-[hsl(var(--foreground))]'
                            "
                        >
                            <component
                                :is="item.meta?.icon"
                                class="size-4 shrink-0"
                                :class="isCollapsed ? '' : 'mr-3'"
                            />
                            <span v-if="!isCollapsed" class="truncate">
                                {{ t(String(item.meta?.titleKey ?? '')) }}
                            </span>
                        </router-link>
                    </nav>
                    <nav class="mt-auto px-2 py-1 w-full">
                        <router-link
                            v-for="item in bottomNavItems"
                            :key="item.path"
                            :to="item.path"
                            class="mb-1 flex h-8 items-center rounded-md px-2.5 text-[13px] transition-colors"
                            :class="
                                route.path.startsWith(`/${item.path}`)
                                    ? 'bg-[hsl(var(--sidebar-accent))] font-medium text-[hsl(var(--sidebar-accent-foreground))]'
                                    : 'text-[hsl(var(--muted-foreground))] hover:bg-[hsl(var(--accent)/0.72)] hover:text-[hsl(var(--foreground))]'
                            "
                        >
                            <component
                                :is="item.meta?.icon"
                                class="size-4 shrink-0"
                                :class="isCollapsed ? '' : 'mr-3'"
                            />
                            <span v-if="!isCollapsed" class="truncate">
                                {{ t(String(item.meta?.titleKey ?? '')) }}
                            </span>
                        </router-link>
                    </nav>
                </aside>

                <main
                    class="min-w-0 min-h-0 overflow-hidden bg-[hsl(var(--background))] text-[13px]"
                >
                    <router-view v-slot="{ Component, route: currentRoute }">
                        <KeepAlive>
                            <component
                                :is="Component"
                                :key="
                                    currentRoute.meta.keepAlive
                                        ? currentRoute.path
                                        : currentRoute.fullPath
                                "
                            />
                        </KeepAlive>
                    </router-view>
                </main>
            </div>
        </div>
    </TooltipProvider>
</template>
