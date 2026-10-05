import { BookOutlined, EditOutlined, ImportOutlined, TeamOutlined } from '@ant-design/icons'
import { Alert, Button, Card, Col, Collapse, Descriptions, Empty, List, Row, Space, Statistic, Tag, Typography } from 'antd'
import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'

import { errorMessage, request } from '../api/client'
import { projectApi } from '../api/projects'
import type { Project } from '../api/types'

interface HealthReport {
  status: 'ok' | 'degraded'
  version: string
  env: string
  uptime_sec: number
  checks: Record<string, { status: string; detail?: string }>
}

const CHECK_LABEL: Record<string, string> = {
  ok: '正常',
  error: '异常',
  not_configured: '未配置',
  enabled: '已启用',
  disabled: '未启用',
}

/**
 * 工作台（产品首页）。
 *
 * 2026-10-05 改版：原来首页直接摊开后端健康检查（版本 / 环境 / 运行秒数 / PG / Redis），
 * 那是运维视角的信息，放在用户第一眼看到的地方既看不懂也没用。
 * 现在首页回答「我能做什么、我写到哪了」，服务状态收进底部「开发者信息」折叠区 ——
 * 出问题时仍然一眼可见。
 */
export default function Dashboard() {
  const [projects, setProjects] = useState<Project[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [health, setHealth] = useState<HealthReport | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await projectApi.list({ page: 1, page_size: 100 })
      setProjects(res.items)
      setError(null)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setLoading(false)
    }
    // 服务状态只是"顺带看一眼"的信息：失败也不打扰首页
    request<HealthReport>('/health')
      .then(setHealth)
      .catch(() => undefined)
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const originals = projects.filter((p) => p.type === 'ORIGINAL')
  const creatives = projects.filter((p) => p.type === 'CREATIVE')
  const recent = [...projects].sort((a, b) => (a.updated_at < b.updated_at ? 1 : -1)).slice(0, 5)

  return (
    <Space direction="vertical" size={16} style={{ display: 'flex' }}>
      <Card>
        <Typography.Title level={4} style={{ marginTop: 0 }}>
          NovaMind 工作台
        </Typography.Title>
        <Typography.Paragraph type="secondary" style={{ marginBottom: 16 }}>
          先把一本原著导进来，再基于它开一篇同人；写作时可以随时让 AI 起草、改写，并用一致性检查盯住人物与世界观。
        </Typography.Paragraph>
        <Space wrap>
          <Link to="/original/overview">
            <Button type="primary" icon={<ImportOutlined />}>
              导入原著
            </Button>
          </Link>
          <Link to="/creative/overview">
            <Button icon={<TeamOutlined />}>开一篇同人</Button>
          </Link>
          <Link to="/creative/chapters">
            <Button icon={<EditOutlined />}>继续写作</Button>
          </Link>
        </Space>
      </Card>

      {error && <Alert type="error" showIcon message="读取文章列表失败" description={error} />}

      <Row gutter={16}>
        <Col span={8}>
          <Card loading={loading}>
            <Statistic title="原著文章" value={originals.length} prefix={<BookOutlined />} />
          </Card>
        </Col>
        <Col span={8}>
          <Card loading={loading}>
            <Statistic title="二创文章" value={creatives.length} prefix={<TeamOutlined />} />
          </Card>
        </Col>
        <Col span={8}>
          <Card loading={loading}>
            <Statistic title="文章总数" value={projects.length} />
          </Card>
        </Col>
      </Row>

      <Card title="最近更新" extra={<Link to="/projects">全部文章 →</Link>} loading={loading}>
        {recent.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有文章，先到「原著 · 总览」导入一本原文" />
        ) : (
          <List
            dataSource={recent}
            renderItem={(item) => (
              <List.Item
                actions={[
                  <Link key="open" to={item.type === 'ORIGINAL' ? '/original/overview' : '/creative/overview'}>
                    打开
                  </Link>,
                ]}
              >
                <List.Item.Meta
                  title={
                    <Space size={6}>
                      <span>{item.name}</span>
                      <Tag color={item.type === 'ORIGINAL' ? 'blue' : 'purple'}>
                        {item.type === 'ORIGINAL' ? '原著' : '二创'}
                      </Tag>
                    </Space>
                  }
                  description={`最近更新：${new Date(item.updated_at).toLocaleString('zh-CN', { hour12: false })}`}
                />
              </List.Item>
            )}
          />
        )}
      </Card>

      <Collapse
        size="small"
        items={[
          {
            key: 'dev',
            label: `开发者信息（服务状态${health ? `：${health.status === 'ok' ? '正常' : '降级'}` : ''}）`,
            children: health ? (
              <Descriptions column={2} size="small">
                <Descriptions.Item label="后端版本">{health.version}</Descriptions.Item>
                <Descriptions.Item label="运行环境">{health.env}</Descriptions.Item>
                <Descriptions.Item label="已运行">{health.uptime_sec} 秒</Descriptions.Item>
                <Descriptions.Item label="依赖">
                  <Space wrap>
                    {Object.entries(health.checks).map(([name, check]) => (
                      <Tag
                        key={name}
                        color={check.status === 'ok' ? 'green' : check.status === 'error' ? 'red' : 'default'}
                      >
                        {name}: {CHECK_LABEL[check.status] ?? check.status}
                      </Tag>
                    ))}
                  </Space>
                </Descriptions.Item>
              </Descriptions>
            ) : (
              <Typography.Text type="secondary">暂时拿不到服务状态（后端可能没起来）</Typography.Text>
            ),
          },
        ]}
      />
    </Space>
  )
}
