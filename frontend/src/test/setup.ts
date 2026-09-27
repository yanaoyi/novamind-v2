import '@testing-library/jest-dom/vitest'

// jsdom 未实现带伪元素的 getComputedStyle，而 antd/rc-util 会用它测量滚动条宽度，
// 每次渲染都刷一堆 "Not implemented" 噪声。这里退化为忽略伪元素参数。
const originalGetComputedStyle = window.getComputedStyle.bind(window)
window.getComputedStyle = ((element: Element) =>
  originalGetComputedStyle(element)) as typeof window.getComputedStyle

// antd 组件（Table / Modal）依赖 matchMedia 与 ResizeObserver，jsdom 未实现，这里补上
if (!window.matchMedia) {
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia
}

if (!globalThis.ResizeObserver) {
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  } as unknown as typeof ResizeObserver
}
