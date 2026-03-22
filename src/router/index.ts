import { createRouter, createWebHistory } from 'vue-router'
import MainLayout from '@/layouts/MainLayout.vue'
import PackagesPage from '@/pages/Packages.vue'
import SearchPage from '@/pages/Search.vue'
import SettingsPage from '@/pages/Settings.vue'
import { Package, Search, Settings } from 'lucide-vue-next'

const router = createRouter({
    history: createWebHistory(),
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
                        title: 'Packages',
                    },
                },
                {
                    path: 'search',
                    component: SearchPage,
                    meta: {
                        icon: Search,
                        title: 'Search',
                    },
                },
                {
                    path: 'settings',
                    component: SettingsPage,
                    meta: {
                        icon: Settings,
                        title: 'Settings',
                    },
                },
            ],
        },
    ],
})

export default router
