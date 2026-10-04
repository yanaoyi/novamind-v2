import { errorMessage } from '../../api/client'
import { CheckCircleOutlined, ReloadOutlined } from '@ant-design/icons'
import {
  Alert,
  Button,
  Card,
  Empty,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'

import { pollTask } from '../../api/taskPoll'
import { writingApi } from '../../api/writing'
import {
  ISSUE_SEVERITY_COLOR,
  ISSUE_SEVERITY_LABEL,
  ISSUE_STATUS_LABEL,
  ISSUE_TYPE_LABEL,
  type ConsistencyIssue,
  type IssueStatus,
} from '../../api/types'
import { useCreativeWorks } from './useCreativeWorks'

const STATUS_COLOR: Record<IssueStatus, string> = {
  OPEN: 'red',
  RESOLVED: 'green',
  IGNORED: 'default',
}

export default function ConsistencyPage() {
  const { originalWorkId, works, workId, setWorkId, loading: worksLoading } = useCreativeWorks()

  const [issues, setIssues] = useState<ConsistencyIssue[]>([])
  const [total, setTotal] = useState(0)
  const [status, setStatus] = useState<IssueStatus | ''>('OPEN')
  const [loading, setLoading] = useState(false)
  const [checking, setChecking] = useState(false)

  const load = useCallback(async () => {
    if (!workId) {
      setIssues([])
      setTotal(0)
      return
    }
    setLoading(true)
    try {
      const res = await writingApi.listIssues(workId, status)
      setIssues(res.items)
      setTotal(res.total)
    } catch (err) {
      message.error(errorMessage(err))
    } finally {
      setLoading(false)
    }
  }, [workId, status])

  useEffect(() => {
    void load()
  }, [load])

  const runCheck = async () => {
    if (!workId) return
    setChecking(true)
    try {
      const task = await writingApi.checkConsistency(workId)
      const finished = await pollTask(task.id)
      if (finished.status !== 'COMPLETED') throw new Error(finished.error || '一致性检查失败')
      message.success('一致性检查完成')
      await load()
    } catch (err) {
      message.error(errorMessage(err))
    } finally {
      setChecking(false)
    }
  }

  const update = async (id: string, next: IssueStatus) => {
    try {
      await writingApi.updateIssue(id, next)
      message.success(`已标记为「${ISSUE_STATUS_LABEL[next]}」`)
      await load()
    } catch (err) {
      message.error(errorMessage(err))
    }
  }

  const columns: ColumnsType<ConsistencyIssue> = [
    {
      title: '严重度',
      dataIndex: 'severity',
      width: 90,
      render: (v: ConsistencyIssue['severity']) => (
        <Tag color={ISSUE_SEVERITY_COLOR[v]}>{ISSUE_SEVERITY_LABEL[v]}</Tag>
      ),
    },
    {
      title: '类型',
      dataIndex: 'type',
      width: 90,
      render: (v: ConsistencyIssue['type']) => <Tag>{ISSUE_TYPE_LABEL[v] ?? v}</Tag>,
    },
    { title: '问题', dataIndex: 'description' },
    {
      title: '依据',
      dataIndex: 'evidence',
      width: 200,
      render: (v: string) => (
        <Tooltip title={v}>
          <Typography.Text type="secondary" ellipsis style={{ maxWidth: 180 }}>
            {v || '—'}
          </Typography.Text>
        </Tooltip>
      ),
    },
    {
      title: '建议',
      dataIndex: 'suggestion',
      width: 220,
      render: (v: string) => <Typography.Text type="secondary">{v || '—'}</Typography.Text>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      render: (v: IssueStatus) => <Tag color={STATUS_COLOR[v]}>{ISSUE_STATUS_LABEL[v]}</Tag>,
    },
    {
      title: '操作',
      width: 170,
      render: (_, row) => (
        <Space size="small">
          {row.status !== 'RESOLVED' && (
            <Button size="small" type="link" onClick={() => void update(row.id, 'RESOLVED')}>
              已解决
            </Button>
          )}
          {row.status !== 'IGNORED' && (
            <Button size="small" type="link" onClick={() => void update(row.id, 'IGNORED')}>
              忽略
            </Button>
          )}
          {row.status !== 'OPEN' && (
            <Button size="small" type="link" onClick={() => void update(row.id, 'OPEN')}>
              重新打开
            </Button>
          )}
        </Space>
      ),
    },
  ]

  if (!originalWorkId) {
    return (
      <Card title="一致性检查">
        <Alert
          type="warning"
          showIcon
          message="还没有选中原著"
          description={
            <span>
              请先到 <Link to="/original/overview">原著总览</Link> 选择原著，再在二创作品上跑一致性检查。
            </span>
          }
        />
      </Card>
    )
  }

  return (
    <Card
      title="一致性检查（§39）"
      extra={
        <Space wrap>
          <Select
            style={{ minWidth: 220 }}
            loading={worksLoading}
            value={workId ?? undefined}
            placeholder="选择二创作品"
            onChange={(v) => setWorkId(v)}
            options={works.map((w) => ({ value: w.id, label: w.title }))}
          />
          <Select
            style={{ width: 130 }}
            value={status}
            onChange={(v) => setStatus(v as IssueStatus | '')}
            options={[
              { value: 'OPEN', label: '待处理' },
              { value: 'RESOLVED', label: '已解决' },
              { value: 'IGNORED', label: '已忽略' },
              { value: '', label: '全部' },
            ]}
          />
          <Button icon={<ReloadOutlined />} onClick={() => void load()}>
            刷新
          </Button>
          <Button
            type="primary"
            icon={<CheckCircleOutlined />}
            loading={checking}
            onClick={() => void runCheck()}
            disabled={!workId}
          >
            开始检查
          </Button>
        </Space>
      }
    >
      {!worksLoading && works.length === 0 && (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="这部原著还没有二创作品，先到「二创 · 总览」创建一个。"
        />
      )}
      <Table
        rowKey="id"
        size="small"
        loading={loading}
        columns={columns}
        dataSource={issues}
        pagination={false}
        locale={{ emptyText: <Empty description="没有该状态的问题" /> }}
      />
      <Typography.Paragraph type="secondary" style={{ marginTop: 8 }}>
        共 {total} 条
      </Typography.Paragraph>
    </Card>
  )
}
