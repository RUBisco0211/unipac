import { ref, onMounted } from 'vue'

export type Theme = 'light' | 'dark' | 'auto'

const theme = ref<Theme>('auto')
let initialized = false

const applyTheme = (newTheme: Theme) => {
    const isDark =
        newTheme === 'dark' ||
        (newTheme === 'auto' && window.matchMedia('(prefers-color-scheme: dark)').matches)

    if (isDark) {
        document.documentElement.classList.add('dark')
    } else {
        document.documentElement.classList.remove('dark')
    }
}

export function useTheme() {
    const setTheme = (newTheme: Theme) => {
        theme.value = newTheme
        localStorage.setItem('unipac-theme', newTheme)
        applyTheme(newTheme)
    }

    onMounted(() => {
        if (initialized) return
        initialized = true

        const savedTheme = localStorage.getItem('unipac-theme') as Theme | null
        if (savedTheme) {
            theme.value = savedTheme
        }
        applyTheme(theme.value)

        window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
            if (theme.value === 'auto') {
                applyTheme('auto')
            }
        })
    })

    return {
        theme,
        setTheme,
    }
}
