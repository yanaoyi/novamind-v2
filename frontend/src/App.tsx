import { Layout, Menu, Typography } from 'antd'
import { Link, Navigate, Route, Routes, useLocation } from 'react-router-dom'

import Dashboard from './pages/Dashboard'
import Placeholder from './pages/Placeholder'
import ProjectsPage from './pages/ProjectsPage'
import SettingsPage from './pages/SettingsPage'
import TasksPage from './pages/TasksPage'
import CreativePage from './pages/CreativePage'
import TokenGate from './components/TokenGate'
import ConsistencyPage from './pages/creative/ConsistencyPage'
import CreativeOverviewPage from './pages/creative/CreativeOverviewPage'
import OutlinePage from './pages/creative/OutlinePage'
import WritingWorkspacePage from './pages/creative/WritingWorkspacePage'
import ChapterReaderPage from './pages/original/ChapterReaderPage'
import OriginalAnalysisPage from './pages/original/OriginalAnalysisPage'
import OriginalCharactersPage from './pages/original/OriginalCharactersPage'
import OriginalChaptersPage from './pages/original/OriginalChaptersPage'
import OriginalOverviewPage from './pages/original/OriginalOverviewPage'
import OriginalPlotPage from './pages/original/OriginalPlotPage'
import OriginalTimelinePage from './pages/original/OriginalTimelinePage'
import OriginalWorldPage from './pages/original/OriginalWorldPage'

const { Header, Sider, Content } = Layout

/** 原著侧页面（PRODUCT_SPEC §8） */
const ORIGINAL_PAGES: Array<{ path: string; label: string; phase: string }> = [
  { path: 'overview', label: '总览', phase: 'Phase 2' },
  { path: 'chapters', label: '章节', phase: 'Phase 2' },
  { path: 'analysis', label: 'AI 分析', phase: 'Phase 3' },
  { path: 'characters', label: '人物', phase: 'Phase 2' },
  { path: 'relationships', label: '人物关系', phase: 'Phase 2' },
  { path: 'world', label: '世界观', phase: 'Phase 2' },
  { path: 'locations', label: '地点', phase: 'Phase 2' },
  { path: 'factions', label: '势力', phase: 'Phase 2' },
  { path: 'plot', label: '剧情', phase: 'Phase 2' },
  { path: 'timeline', label: '时间线', phase: 'Phase 2' },
  { path: 'knowledge', label: '知识库', phase: 'Phase 3' },
]

/** 二创侧页面 */
const CREATIVE_PAGES: Array<{ path: string; label: string; phase: string }> = [
  { path: 'overview', label: '总览', phase: 'Phase 4' },
  { path: 'settings', label: '设定', phase: 'Phase 4' },
  { path: 'characters', label: '人物', phase: 'Phase 4' },
  { path: 'world', label: '世界观', phase: 'Phase 4' },
  { path: 'plot', label: '剧情', phase: 'Phase 4' },
  { path: 'timeline', label: '时间线', phase: 'Phase 4' },
  { path: 'outline', label: '大纲', phase: 'Phase 5' },
  { path: 'chapters', label: '章节', phase: 'Phase 5' },
  { path: 'materials', label: '素材', phase: 'Phase 5' },
  { path: 'mappings', label: '映射关系', phase: 'Phase 4' },
]

const SIMPLE_PAGES: Array<{ path: string; label: string; phase?: string }> = [
  { path: '/creative-workspace', label: '二创工作区' },
  { path: '/editor', label: '编辑器', phase: 'Phase 5' },
  { path: '/ai', label: 'AI 助手', phase: 'Phase 3' },
  { path: '/consistency', label: '一致性检查', phase: 'Phase 6' },
  { path: '/tasks', label: '任务中心' },
  { path: '/settings', label: '模型设置' },
]

function buildMenu() {
  return [
    { key: '/dashboard', label: <Link to="/dashboard">工作台</Link> },
    { key: '/projects', label: <Link to="/projects">文章管理</Link> },
    {
      key: 'original',
      label: '原著',
      children: ORIGINAL_PAGES.map((p) => ({
        key: `/original/${p.path}`,
        label: <Link to={`/original/${p.path}`}>{p.label}</Link>,
      })),
    },
    {
      key: 'creative',
      label: '二创',
      children: CREATIVE_PAGES.map((p) => ({
        key: `/creative/${p.path}`,
        label: <Link to={`/creative/${p.path}`}>{p.label}</Link>,
      })),
    },
    ...SIMPLE_PAGES.map((p) => ({
      key: p.path,
      label: <Link to={p.path}>{p.label}</Link>,
    })),
  ]
}

/** 所有可高亮的菜单 key（用于按前缀匹配，见下方 selectedKeys） */
const ALL_MENU_KEYS = [
  '/dashboard',
  '/projects',
  ...ORIGINAL_PAGES.map((p) => `/original/${p.path}`),
  ...CREATIVE_PAGES.map((p) => `/creative/${p.path}`),
  ...SIMPLE_PAGES.map((p) => p.path),
]

/**
 * 按"最长前缀"决定菜单高亮。
 *
 * 之前用 `selectedKeys={[location.pathname]}`：像 /original/chapters/3 这种详情路由
 * 在菜单里没有精确匹配项，于是整条菜单都不高亮，使用者会以为自己"不在任何板块"（审查 P2）。
 */
function selectedMenuKeys(pathname: string): string[] {
  const matched = ALL_MENU_KEYS.filter((key) => pathname === key || pathname.startsWith(`${key}/`))
  if (matched.length === 0) return [pathname]
  return [matched.reduce((longest, key) => (key.length > longest.length ? key : longest))]
}

export default function App() {
  const location = useLocation()
  const selectedKeys = selectedMenuKeys(location.pathname)

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <TokenGate />
      <Header style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <Typography.Title level={4} style={{ color: '#fff', margin: 0 }}>
          NovaMind
        </Typography.Title>
        <Typography.Text style={{ color: 'rgba(255,255,255,0.65)' }}>
          AI 小说创作系统 · Phase 1
        </Typography.Text>
      </Header>

      <Layout>
        <Sider width={200} theme="light">
          <Menu
            mode="inline"
            selectedKeys={selectedKeys}
            defaultOpenKeys={['original', 'creative']}
            style={{ height: '100%', borderRight: 0 }}
            items={buildMenu()}
          />
        </Sider>

        <Content style={{ padding: 16 }}>
          <Routes>
            <Route path="/" element={<Navigate to="/dashboard" replace />} />
            <Route path="/dashboard" element={<Dashboard />} />
            <Route path="/projects" element={<ProjectsPage />} />

            <Route path="/original" element={<Navigate to="/original/overview" replace />} />
            <Route path="/original/overview" element={<OriginalOverviewPage />} />
            <Route path="/original/chapters" element={<OriginalChaptersPage />} />
            <Route path="/original/chapters/:no" element={<ChapterReaderPage />} />
            <Route path="/original/characters" element={<OriginalCharactersPage />} />
            {/* 三个入口复用同一个世界观页，用 key 强制重挂载以切到对应标签页 */}
            <Route path="/original/world" element={<OriginalWorldPage key="world" defaultTab="rules" />} />
            <Route path="/original/locations" element={<OriginalWorldPage key="locations" defaultTab="locations" />} />
            <Route path="/original/factions" element={<OriginalWorldPage key="factions" defaultTab="factions" />} />
            <Route path="/original/timeline" element={<OriginalTimelinePage />} />
            <Route path="/original/plot" element={<OriginalPlotPage />} />
            <Route path="/original/analysis" element={<OriginalAnalysisPage />} />

            {ORIGINAL_PAGES.filter(
              (p) =>
                ![
                  'overview',
                  'chapters',
                  'characters',
                  'world',
                  'locations',
                  'factions',
                  'timeline',
                  'plot',
                  'analysis',
                ].includes(p.path),
            ).map((p) => (
              <Route
                key={p.path}
                path={`/original/${p.path}`}
                element={<Placeholder title={`原著 · ${p.label}`} phase={p.phase} />}
              />
            ))}

            {CREATIVE_PAGES.map((p) => (
              <Route
                key={p.path}
                path={`/creative/${p.path}`}
                element={
                  p.path === 'overview' ? (
                    <CreativeOverviewPage key="creative-overview" />
                  ) : p.path === 'outline' ? (
                    <OutlinePage key="creative-outline" />
                  ) : p.path === 'chapters' ? (
                    <WritingWorkspacePage key="creative-chapters" defaultTab="chapters" />
                  ) : ['characters', 'world', 'timeline', 'mappings'].includes(p.path) ? (
                    <CreativePage
                      key={`creative-${p.path}`}
                      defaultTab={p.path as 'characters' | 'world' | 'timeline' | 'mappings'}
                    />
                  ) : (
                    <Placeholder title={`二创 · ${p.label}`} phase={p.phase} />
                  )
                }
              />
            ))}
            <Route path="/creative" element={<Navigate to="/creative/overview" replace />} />
            <Route path="/creative-workspace" element={<CreativePage key="creative-workspace" />} />
            <Route path="/editor" element={<WritingWorkspacePage key="editor" />} />
            <Route path="/consistency" element={<ConsistencyPage />} />

            {SIMPLE_PAGES.map((p) => (
              ['/editor', '/consistency'].includes(p.path) ? null : (
                <Route key={p.path} path={p.path} element={<Placeholder title={p.label} phase={p.phase} />} />
              )
            ))}
            <Route path="/tasks" element={<TasksPage />} />
            <Route path="/settings" element={<SettingsPage />} />

            <Route path="*" element={<Placeholder title="页面不存在" />} />
          </Routes>
        </Content>
      </Layout>
    </Layout>
  )
}
