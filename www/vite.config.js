import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

const backendPort = process.env.BACKEND_PORT || '8080'

export default defineConfig({
    plugins: [vue()],
    resolve: {
        alias: {
            '@': path.resolve(__dirname, './src'),
        },
    },
    base: './',
    server: {
        host: '127.0.0.1',
        port: 5173,
        proxy: {
            '/api': {
                target: `http://127.0.0.1:${backendPort}`,
                changeOrigin: true
            }
        }
    },
    build: {
        outDir: 'dist',
        assetsDir: 'assets',
        sourcemap: false,
    },
})

