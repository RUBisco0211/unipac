import { createRouter, createWebHashHistory } from 'vue-router'
import MainLayout from '@/layouts/MainLayout.vue'
import PackagesPage from '@/pages/Packages.vue'
import OutdatedPage from '@/pages/Outdated.vue'
import SearchPage from '@/pages/Search.vue'
import SettingsPage from '@/pages/Settings.vue'
import { Package, Settings, RefreshCcw, Search } from 'lucide-vue-next'

const router = createRouter({
    history: createWebHashHistory(),
    routes: [
        {
            path: '/',
            redirect: '/packages',
            component: MainLayout,
            children: [
                {
                    path: 'packages',
                    component: PackagesPage,
                    meta: {
                        icon: Package,
                        titleKey: 'nav.packages',
                        keepAlive: true,
                        position: 'top',
                    },
                },
                {
                    path: 'outdated',
                    component: OutdatedPage,
                    meta: {
                        icon: RefreshCcw,
                        titleKey: 'nav.outdated',
                        keepAlive: true,
                        position: 'top',
                    },
                },
                {
                    path: 'search',
                    component: SearchPage,
                    meta: {
                        icon: Search,
                        titleKey: 'nav.search',
                        keepAlive: true,
                        position: 'top',
                    },
                },
                {
                    path: 'settings',
                    component: SettingsPage,
                    meta: {
                        icon: Settings,
                        titleKey: 'nav.settings',
                        keepAlive: true,
                        position: 'bottom',
                    },
                },
            ],
        },
        {
            path: '/:pathMatch(.*)*',
            redirect: '/packages',
        },
    ],
})

export default router
