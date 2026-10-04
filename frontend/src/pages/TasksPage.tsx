import { ReloadOutlined } from '@ant-design/icons'
import {
  Button,
  Card,
  Empty,
  Popconfirm,
  Progress,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useCallback, useEffect, useRef, useState } from 'react'

import { taskApi } from '../api/ai'
import {
  TASK_STATUS_COLOR,
  TASK_STATUS_LABEL,
  type Task,
  type TaskStatus,
} from '../api/types'

const ACTIVE_STATUSES: TaskStatus[] = ['PENDING', 'RUNNING', 'PAUSED']

export default function TasksPage() {
  const [tasks, setTasks] = useState<Task[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [statusFilter, setStatusFilter] = useState<TaskStatus | undefined>(undefined)
  const timerRef = useRef<number | null>(null)

  const load = useCallback(async () => {
    try {
      const data = await taskApi.list({ status: statusFilter, pageSize: 50 })
      setTasks(data.items)
      setTotal(data.total)
    } catch (err) {
      message.error((err as Error).message)
    }
  }, [statusFilter])

  const reload = useCallback(async () => {
    setLoading(true)
    await load()
    setLoading(false)
  }, [load])

  useEffect(() => {
    void reload()
  }, [reload])

  // 有活动任务时自动刷新，任务跑完自动停
  useEffect(() => {
    const hasActive = tasks.some((t) => ACTIVE_STATUSES.includes(t.status))
    if (!hasActive) {
      if (timerRef.current) {
        window.clearInterval(timerRef.current)
        timerRef.current = null
      }
      return
    }
    if (timerRef.current) return
    timerRef.current = window.setInterval(() => {
      void load()
    }, 2000)
    return () => {
      if (timerRef.current) {
        window.clearInterval(timerRef.current)
        timerRef.current = null
      }
    }
  }, [tasks, load])

  const columns: ColumnsType<Task> = [
    {
      title: '任务',
      dataIndex: 'type',
      key: 'type',
      width: 240,
      render: (value: string, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{value}</Typography.Text>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {new Date(record.created_at).toLocaleString('zh-CN')}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 110,
      render: (value: TaskStatus) => <Tag color={TASK_STATUS_COLOR[value]}>{TASK_STATUS_LABEL[value]}</Tag>,
    },
    {
      title: '进度',
      key: 'progress',
      width: 240,
      render: (_, record) => (
        <Space direction="vertical" size={0} style={{ width: '100%' }}>
          <Progress
            percent={record.progress}
            size="small"
            status={
              record.status === 'FAILED'
                ? 'exception'
                : record.status === 'COMPLETED'
                  ? 'success'
                  : ACTIVE_STATUSES.includes(record.status)
                    ? 'active'
                    : 'normal'
            }
          />
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {record.progress_message || '—'}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: '尝试',
      key: 'attempts',
      width: 80,
      render: (_, record) => `${record.attempts}/${record.max_attempts}`,
    },
    {
      title: '结果 / 错误',
      key: 'result',
      ellipsis: true,
      render: (_, record) => {
        if (record.error) {
          return (
            <Tooltip title={record.error}>
              <Typography.Text type="danger">{record.error}</Typography.Text>
            </Tooltip>
          )
        }
        // output 可能为 null（后端初始值是 {}，但历史数据/失败任务可能没有），
        // 直接解引用会让整张表崩掉（审查 P2）
        const output = (record.output ?? {}) as { summary?: unknown; proposals_created?: unknown }
        const summary = output.summary
        const created = output.proposals_created
        return (
          <Typography.Text type="secondary">
            {typeof summary === 'string' ? summary : created !== undefined ? `产出提案 ${created} 条` : '—'}
          </Typography.Text>
        )
      },
    },
    {
      title: '操作',
      key: 'actions',
      width: 150,
      render: (_, record) => (
        <Space>
          {ACTIVE_STATUSES.includes(record.status) && (
            <Popconfirm
              title="取消该任务？"
              okText="确认取消"
              cancelText="取消"
              onConfirm={async () => {
                try {
                  await taskApi.cancel(record.id)
                  message.success('已取消')
                  await load()
                } catch (err) {
                  message.error((err as Error).message)
                }
              }}
            >
              <Button type="link" size="small" danger>
                取消
              </Button>
            </Popconfirm>
          )}
          {(record.status === 'FAILED' || record.status === 'CANCELLED') && (
            <Button
              type="link"
              size="small"
              onClick={async () => {
                try {
                  await taskApi.retry(record.id)
                  message.success('已重新排队')
                  await load()
                } catch (err) {
                  message.error((err as Error).message)
                }
              }}
            >
              重试
            </Button>
          )}
        </Space>
      ),
    },
  ]

  return (
    <Card
      title="任务中心"
      extra={
        <Space>
          <Select
            allowClear
            placeholder="全部状态"
            style={{ width: 160 }}
            value={statusFilter}
            onChange={(value) => setStatusFilter(value)}
            options={(Object.keys(TASK_STATUS_LABEL) as TaskStatus[]).map((s) => ({
              value: s,
              label: TASK_STATUS_LABEL[s],
            }))}
          />
          <Button icon={<ReloadOutlined />} onClick={() => void reload()}>
            刷新
          </Button>
        </Space>
      }
    >
      {tasks.length === 0 && !loading ? (
        <Empty description="还没有任务。导入原著后可以在「原著 · AI 分析」里触发分析。" />
      ) : (
        <Table<Task>
          rowKey="id"
          loading={loading}
          dataSource={tasks}
          columns={columns}
          pagination={{ pageSize: 20, total, showTotal: (t) => `共 ${t} 个任务` }}
          expandable={{
            expandedRowRender: (record) => (
              <Space direction="vertical" size={4} style={{ display: 'flex' }}>
                <Typography.Text type="secondary">任务 ID：{record.id}</Typography.Text>
                <Typography.Text type="secondary">入参：{JSON.stringify(record.input)}</Typography.Text>
                <Typography.Text type="secondary">输出：{JSON.stringify(record.output)}</Typography.Text>
                {record.error && <Typography.Text type="danger">错误：{record.error}</Typography.Text>}
              </Space>
            ),
          }}
        />
      )}
    </Card>
  )
}
