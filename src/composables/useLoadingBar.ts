import { computed, ref } from 'vue'

const loadingCount = ref(0)

export function startLoadingBar() {
    loadingCount.value += 1
}

export function finishLoadingBar() {
    loadingCount.value = Math.max(0, loadingCount.value - 1)
}

export function useLoadingBar() {
    const isLoading = computed(() => loadingCount.value > 0)

    return {
        isLoading,
    }
}
