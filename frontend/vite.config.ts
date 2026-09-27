import react from '@vitejs/plugin-react'
import { defineConfig } from 'vitest/config'

export default defineConfig({
  plugins: [react()],
  server: {
    // 绑 0.0.0.0：懒猫微服把服务发布出去时，代理从容器内/外连都可能，
    // 只绑 127.0.0.1 会出现 "upstream is not accepting connections" 之类的问题。
    host: true,
    port: 5173,
    strictPort: true,
    // 开发时把 /api 代理到后端，避免跨域配置
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    css: false,
    // antd 组件在 jsdom 下的首次渲染偏慢，放宽超时避免误报
    testTimeout: 20000,
  },
})
