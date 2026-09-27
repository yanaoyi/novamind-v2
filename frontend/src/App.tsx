import { Layout, Menu, Typography } from 'antd'
import { Link, Navigate, Route, Routes, useLocation } from 'react-router-dom'

import Dashboard from './pages/Dashboard'
import Placeholder from './pages/Placeholder'
import ProjectsPage from './pages/ProjectsPage'

const { Header, Sider, Content } = Layout

/** 原著侧页面（PRODUCT_SPEC §8） */
const ORIGINAL_PAGES: Array<{ path: string; label: string; phase: string }> = [
  { path: 'overview', label: '总览', phase: 'Phase 2' },
  { path: 'chapters', label: '章节', phase: 'Phase 2' },
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
  { path: '/editor', label: '编辑器', phase: 'Phase 5' },
  { path: '/ai', label: 'AI 助手', phase: 'Phase 3' },
  { path: '/consistency', label: '一致性检查', phase: 'Phase 6' },
  { path: '/tasks', label: '任务中心', phase: 'Phase 3' },
  { path: '/settings', label: '设置', phase: 'Phase 3' },
]

function buildMenu() {
  return [
    { key: '/dashboard', label: <Link to="/dashboard">工作台</Link> },
    { key: '/projects', label: <Link to="/projects">工程管理</Link> },
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

export default function App() {
  const location = useLocation()

  return (
    <Layout style={{ minHeight: '100vh' }}>
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
            selectedKeys={[location.pathname]}
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

            {ORIGINAL_PAGES.map((p) => (
              <Route
                key={p.path}
                path={`/original/${p.path}`}
                element={<Placeholder title={`原著 · ${p.label}`} phase={p.phase} />}
              />
            ))}
            <Route path="/original" element={<Navigate to="/original/overview" replace />} />

            {CREATIVE_PAGES.map((p) => (
              <Route
                key={p.path}
                path={`/creative/${p.path}`}
                element={<Placeholder title={`二创 · ${p.label}`} phase={p.phase} />}
              />
            ))}
            <Route path="/creative" element={<Navigate to="/creative/overview" replace />} />

            {SIMPLE_PAGES.map((p) => (
              <Route
                key={p.path}
                path={p.path}
                element={<Placeholder title={p.label} phase={p.phase} />}
              />
            ))}

            <Route path="*" element={<Placeholder title="页面不存在" />} />
          </Routes>
        </Content>
      </Layout>
    </Layout>
  )
}
