import type { LucideProps } from 'lucide-vue-next'
import type { FunctionalComponent } from 'vue'

declare module 'vue-router' {
    interface RouteMeta {
        icon?: FunctionalComponent<LucideProps, {}, any, {}>
        title?: string
        keepAlive?: boolean
        position?: 'top' | 'bottom'
    }
}
