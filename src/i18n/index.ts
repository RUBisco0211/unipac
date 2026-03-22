import { createI18n } from 'vue-i18n'
import { messages, type AppLocale } from './messages'

function resolveLocale(): AppLocale {
    const preferred = [...(navigator.languages ?? []), navigator.language].filter(Boolean)

    for (const locale of preferred) {
        const normalized = locale.toLowerCase()
        if (normalized.startsWith('zh')) {
            return 'zh-CN'
        }
        if (normalized.startsWith('en')) {
            return 'en'
        }
    }

    return 'en'
}

export const i18n = createI18n({
    legacy: false,
    locale: resolveLocale(),
    fallbackLocale: 'en',
    messages,
})

export function setLocale(locale: AppLocale) {
    i18n.global.locale.value = locale
}
