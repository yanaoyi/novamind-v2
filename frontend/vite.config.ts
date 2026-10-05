import react from '@vitejs/plugin-react'
import { loadEnv } from 'vite'
import { defineConfig } from 'vitest/config'

/**
 * 本地开发时从 backend/.env 读取访问令牌（ADMIN_TOKEN），由**代理**注入到 /api 请求里。
 *
 * 为什么这么做（2026-10-05 按 BOSS 反馈加）：
 *   后端所有业务接口都要求 `Authorization: Bearer <ADMIN_TOKEN>`（安全基线），
 *   但本地开发是同一台机器、同一个人，不该让使用者每次手动把令牌粘进浏览器
 *   （粘进 localStorage 反而更容易被同源脚本读到）。放在开发代理里注入：
 *     - 浏览器侧零配置，打开就能用；
 *     - 令牌不进前端产物、不进浏览器存储；
 *     - 后端仍然要求令牌，局域网里其它人照样过不去；
 *     - 生产环境（nginx 反代）不做这个注入，仍由使用者在界面上填一次。
 */
function devAdminToken(mode: string): string {
  // 用 vite 自带的 loadEnv 读 backend/.env：不必引 node:fs，也就不需要 @types/node
  try {
    return loadEnv(mode, '../backend', '').ADMIN_TOKEN?.trim() ?? ''
  } catch {
    return ''
  }
}

export default defineConfig(({ mode }) => ({
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
        configure: (proxy) => {
          const token = devAdminToken(mode)
          if (!token) return
          // 代理侧覆盖 Authorization：即便浏览器里存着旧令牌，也会被这里的正确值顶掉
          // （vite 内置的 http-proxy 类型声明里没有 on()，这里显式收窄成运行时真正用到的形状）
          const server = proxy as unknown as {
            on: (event: string, cb: (req: { setHeader: (k: string, v: string) => void }) => void) => void
          }
          server.on('proxyReq', (proxyReq) => {
            proxyReq.setHeader('Authorization', `Bearer ${token}`)
          })
        },
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    css: false,
    // antd 组件在 jsdom 下偏慢：人物页那张 11 维度 DNA 表单实测单例要 19-20 秒，
    // 与其它测试文件并行跑时会超过 20 秒。放宽到 45 秒避免误报超时。
    testTimeout: 45000,
  },
}))
