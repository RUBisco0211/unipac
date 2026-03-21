<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import TitleBar from '@/components/common/TitleBar.vue'
import { ChevronLeft, ChevronRight, Package, Search, Settings } from 'lucide-vue-next'
import { useRouter } from 'vue-router'

const AUTO_COLLAPSE_WIDTH = 1024
const AUTO_EXPAND_WIDTH = 1180

const router = useRouter()
const route = router.currentRoute
const isCollapsed = ref(false)

const navItems = computed(() => {
  return route.value.matched[0].children
})

const sidebarClasses = computed(() => {
  return isCollapsed.value ? 'w-20' : 'w-64'
})

const toggleLabel = computed(() => {
  return isCollapsed.value ? 'Expand sidebar' : 'Collapse sidebar'
})

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
  <div class="theme-app flex flex-col h-screen w-full font-sans overflow-hidden">
    <title-bar type="hidden" />
    <div class="flex flex-1 overflow-hidden">
      <!-- Sidebar -->
      <aside
        class="flex flex-col shrink-0 transition-[width] duration-200 ease-out"
        :class="sidebarClasses"
      >
        <nav class="flex-1 p-4 space-y-1">
          <router-link
            v-for="item in navItems"
            :key="item.path"
            :to="item.path"
            class="group relative flex items-center rounded-md px-3 py-2 text-sm font-medium transition-colors"
            :class="
              route.path.startsWith(item.path)
                ? 'bg-[var(--color-bg-panel-hover)] theme-text'
                : 'theme-text-secondary hover:bg-[var(--color-bg-panel-hover)] hover:text-[var(--color-text)]'
            "
          >
            <component
              :is="item.meta?.icon"
              class="h-4 w-4 shrink-0"
              :class="isCollapsed ? '' : 'mr-3'"
            />
            <span v-if="!isCollapsed" class="truncate">{{ item.name }}</span>
            <span
              v-else
              class="pointer-events-none absolute left-full top-1/2 z-20 ml-3 -translate-y-1/2 whitespace-nowrap rounded-md border theme-panel theme-border px-2 py-1 text-xs opacity-0 shadow-lg transition-opacity duration-150 group-hover:opacity-100"
            >
              {{ item.name }}
            </span>
          </router-link>
        </nav>
        <div class="theme-border border-t p-4">
          <button
            type="button"
            class="group relative flex w-full items-center rounded-md px-3 py-2 text-sm font-medium transition-colors theme-text-secondary hover:bg-[var(--color-bg-panel-hover)] hover:text-[var(--color-text)]"
            :class="isCollapsed ? 'justify-center' : 'justify-start'"
            :aria-label="toggleLabel"
            @click="toggleSidebar"
          >
            <ChevronRight v-if="isCollapsed" class="h-4 w-4 shrink-0" />
            <ChevronLeft v-else class="h-4 w-4 shrink-0 mr-3" />
            <span v-if="!isCollapsed">{{ toggleLabel }}</span>
            <span
              v-else
              class="pointer-events-none absolute left-full top-1/2 z-20 ml-3 -translate-y-1/2 whitespace-nowrap rounded-md border theme-panel theme-border px-2 py-1 text-xs opacity-0 shadow-lg transition-opacity duration-150 group-hover:opacity-100"
            >
              {{ toggleLabel }}
            </span>
          </button>
        </div>
      </aside>

      <!-- Main Content -->
      <main
        class="theme-app flex-1 flex flex-col min-w-0 overflow-hidden rounded-2xl border theme-border"
      >
        <router-view v-slot="{ Component }">
          <keep-alive>
            <component :is="Component" />
          </keep-alive>
        </router-view>
      </main>
    </div>
  </div>
</template>
