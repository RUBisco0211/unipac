import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'

export function useHistory() {
    const router = useRouter()

    const historyState = ref<Record<string, unknown>>({})
    const canGoBack = computed(() => Boolean(historyState.value.back))
    const canGoForward = computed(() => Boolean(historyState.value.forward))

    let removeAfterEachHook: (() => void) | null = null
    let removePopStateListener: (() => void) | null = null

    function syncHistoryState() {
        historyState.value = (window.history.state as Record<string, unknown> | null) ?? {}
    }

    onMounted(() => {
        syncHistoryState()
        removeAfterEachHook = router.afterEach(() => {
            syncHistoryState()
        })

        const handlePopState = () => {
            syncHistoryState()
        }

        window.addEventListener('popstate', handlePopState)
        removePopStateListener = () => {
            window.removeEventListener('popstate', handlePopState)
        }
    })

    onBeforeUnmount(() => {
        removeAfterEachHook?.()
        removePopStateListener?.()
    })

    return {
        canGoBack,
        canGoForward,
        goBack: () => {
            if (!canGoBack.value) return
            router.back()
        },
        goForward: () => {
             if (!canGoForward.value) return
             router.forward()
        }
    }
}
