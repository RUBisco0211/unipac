<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { RouteRecordRaw, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'

const { isCollapsed } = defineProps<{ isCollapsed: boolean }>()

const router = useRouter()
const route = router.currentRoute

const { t } = useI18n()

const topNavItems = computed(
    () => route.value.matched[0]?.children.filter(i => i.meta?.position === 'top') ?? []
)
const bottomNavItems = computed(
    () => route.value.matched[0]?.children.filter(i => i.meta?.position === 'bottom') ?? []
)

const routeItemClass = computed(() => (item: RouteRecordRaw) => {
    return route.value.path.startsWith(`/${item.path}`)
        ? 'bg-[hsl(var(--sidebar-accent))] font-medium text-[hsl(var(--sidebar-accent-foreground))]'
        : 'text-[hsl(var(--muted-foreground))] hover:bg-[hsl(var(--accent)/0.72)] hover:text-[hsl(var(--foreground))]'
})
</script>
<template>
    <aside
        class="flex min-h-0 flex-col border-r border-[hsl(var(--border))] bg-[hsl(var(--sidebar))] transition-[width] duration-200"
        :class="isCollapsed ? 'w-13' : 'w-48'"
    >
        <nav class="flex flex-1 flex-col px-2 py-2">
            <router-link
                v-for="item in topNavItems"
                :key="item.path"
                :to="item.path"
                class="mb-1 py-4.5 flex h-8 items-center rounded-xl px-2.5 text-[13px] transition-colors"
                :class="routeItemClass(item)"
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
                class="mb-1 flex py-4.5 h-8 items-center rounded-xl px-2.5 text-[13px] transition-colors"
                :class="routeItemClass(item)"
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
</template>
