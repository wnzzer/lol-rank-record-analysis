import { createRouter, createWebHashHistory, RouteRecordRaw } from 'vue-router';

// 定义路由记录
const routes: Array<RouteRecordRaw> = [
    {
        path: '/',
        redirect: '/Loading',
    },
    {
        path: '/Record',
        name: 'Record',
        component: () => import('@renderer/views/Record.vue'), // 懒加载
    },
    {
        path: '/Gaming',
        name: 'Gaming',
        component: () => import('@renderer/views/Gaming.vue'), // 懒加载
    },
    {
        path: '/Loading',
        name: 'Loading',
        component: () => import('@renderer/views/Loading.vue'), // 懒加载
    },
];

// 创建路由实例
const router = createRouter({
    history: createWebHashHistory(), // 使用 WebHashHistory 模式
    routes,
});

export default router;
