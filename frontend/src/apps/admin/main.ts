import App from '@/App.vue'
import { bootstrapApp } from '@/app/bootstrap'
import { setSiteContext } from '@/app/siteContext'
import router, { adminCustomMenuItems } from './router'

setSiteContext({ router, getCustomMenuItems: adminCustomMenuItems })
void bootstrapApp(App, router)
