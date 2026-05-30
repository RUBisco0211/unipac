import { ref, onMounted, onBeforeUnmount } from 'vue'

export function useSidebar() {
    const AUTO_COLLAPSE_WIDTH = 1080
    const AUTO_EXPAND_WIDTH = 1240

    const isCollapsed = ref(false)
    function toggleSidebar() {
        isCollapsed.value = !isCollapsed.value
    }

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

    onMounted(() => {
        syncSidebarWithWidth()
        window.addEventListener('resize', syncSidebarWithWidth)
    })

    onBeforeUnmount(() => {
        window.removeEventListener('resize', syncSidebarWithWidth)
    })

    return {
        isCollapsed,
        toggleSidebar,
    }
}
