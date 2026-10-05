import { Alert, Card, Descriptions, Space, Tag, Typography } from 'antd'
import { useEffect, useState } from 'react'

import { request } from '../api/client'

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
}

const CHECK_COLOR: Record<string, string> = {
  ok: 'green',
  error: 'red',
  not_configured: 'default',
}

export default function Dashboard() {
  const [health, setHealth] = useState<HealthReport | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    request<HealthReport>('/health')
      .then(setHealth)
      .catch((err: Error) => setError(err.message))
  }, [])

  return (
    <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
      <Card title="NovaMind 工作台">
        <Typography.Paragraph type="secondary">
          面向「基于原著进行二次创作」的 AI 写作系统。当前处于 Phase 1（基础框架），
          后端已具备文章管理 API 与数据库迁移能力；原著分析、二创设计、写作与一致性检查按
          Phase 2–6 逐步实现。
        </Typography.Paragraph>
      </Card>

      <Card title="后端服务状态" size="small">
        {error && <Alert type="error" showIcon message={`无法连接后端：${error}`} />}
        {health && (
          <Descriptions column={1} size="small">
            <Descriptions.Item label="总体状态">
              <Tag color={health.status === 'ok' ? 'green' : 'orange'}>
                {health.status === 'ok' ? '正常' : '降级'}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="版本">{health.version}</Descriptions.Item>
            <Descriptions.Item label="环境">{health.env}</Descriptions.Item>
            <Descriptions.Item label="已运行">{health.uptime_sec} 秒</Descriptions.Item>
            <Descriptions.Item label="依赖">
              <Space>
                {Object.entries(health.checks).map(([name, check]) => (
                  <Tag key={name} color={CHECK_COLOR[check.status] ?? 'default'}>
                    {name}: {CHECK_LABEL[check.status] ?? check.status}
                  </Tag>
                ))}
              </Space>
            </Descriptions.Item>
          </Descriptions>
        )}
      </Card>
    </Space>
  )
}
