import { errorMessage } from '../../api/client'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import {
  Button,
  Card,
  Empty,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'

import { eventApi } from '../../api/events'
import { PLOT_ARC_LABELS, type OriginalEvent, type PlotArc, type PlotArcInput, type PlotArcType } from '../../api/types'
import { useOriginalStore } from '../../stores/originalStore'

const ARC_TYPES = Object.keys(PLOT_ARC_LABELS) as PlotArcType[]

export default function OriginalPlotPage() {
  const { workId, work, loadWork } = useOriginalStore()
  const [arcs, setArcs] = useState<PlotArc[]>([])
  const [events, setEvents] = useState<OriginalEvent[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<PlotArc | null>(null)
  const [form] = Form.useForm<PlotArcInput>()

  const reload = useCallback(async () => {
    if (!workId) return
    setLoading(true)
    try {
      const [arcList, eventList] = await Promise.all([
        eventApi.listPlotArcs(workId),
        eventApi.list(workId, { pageSize: 500 }),
      ])
      setArcs(arcList.items)
      setEvents(eventList.items)
    } catch (err) {
      message.error(errorMessage(err))
    } finally {
      setLoading(false)
    }
  }, [workId])

  useEffect(() => {
    if (!workId) return
    if (!work) void loadWork(workId).catch((err: Error) => message.error(err.message))
  }, [workId, work, loadWork])

  useEffect(() => {
    void reload()
  }, [reload])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ type: 'main' })
    setOpen(true)
  }

  const openEdit = (record: PlotArc) => {
    setEditing(record)
    form.setFieldsValue({
      type: record.type,
      title: record.title,
      summary: record.summary,
      start_event_id: record.start_event_id,
      end_event_id: record.end_event_id,
    })
    setOpen(true)
  }

  const submit = async () => {
    if (!workId) return
    const values = await form.validateFields()
    try {
      if (editing) await eventApi.updatePlotArc(editing.id, values)
      else await eventApi.createPlotArc(workId, values)
      message.success(editing ? '剧情弧已更新' : '剧情弧已新增')
      setOpen(false)
      await reload()
    } catch (err) {
      message.error(errorMessage(err))
    }
  }

  const remove = async (record: PlotArc) => {
    try {
      await eventApi.removePlotArc(record.id)
      await reload()
    } catch (err) {
      message.error(errorMessage(err))
    }
  }

  if (!workId) {
    return (
      <Card title="剧情">
        <Empty description={<span>还没有选择原著，<Link to="/original/overview">去选择</Link></span>} />
      </Card>
    )
  }

  const eventTitle = (id: string | null) =>
    id ? (events.find((e) => e.id === id)?.title ?? id.slice(0, 6)) : ''

  const columns: ColumnsType<PlotArc> = [
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 110,
      render: (v: PlotArcType) => <Tag color={v === 'main' ? 'red' : 'blue'}>{PLOT_ARC_LABELS[v] ?? v}</Tag>,
    },
    { title: '标题', dataIndex: 'title', key: 'title', width: 240 },
    { title: '摘要', dataIndex: 'summary', key: 'summary', ellipsis: true },
    {
      title: '起止事件',
      key: 'range',
      width: 260,
      render: (_, record) => (
        <Typography.Text type="secondary">
          {eventTitle(record.start_event_id) || '—'} → {eventTitle(record.end_event_id) || '—'}
        </Typography.Text>
      ),
    },
    {
      title: '操作',
      key: 'actions',
      width: 130,
      render: (_, record) => (
        <Space>
          <Button type="link" size="small" onClick={() => openEdit(record)}>
            编辑
          </Button>
          <Popconfirm title="删除该剧情弧？" okText="确认删除" cancelText="取消" onConfirm={() => remove(record)}>
            <Button type="link" size="small" danger>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  const eventOptions = events.map((e) => ({ value: e.id, label: e.title }))

  return (
    <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
      <Card
        title={work ? `《${work.title}》剧情弧（${arcs.length}）` : '剧情弧'}
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={() => void reload()}>
              刷新
            </Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              新增剧情弧
            </Button>
          </Space>
        }
      >
        {arcs.length === 0 && !loading ? (
          <Empty description="把原著里的主线、支线、人物线梳理出来，后续二创就是在这上面分叉" />
        ) : (
          <Table<PlotArc> rowKey="id" loading={loading} dataSource={arcs} columns={columns} pagination={false} />
        )}
      </Card>

      <Modal
        open={open}
        title={editing ? `编辑剧情弧：${editing.title}` : '新增剧情弧'}
        okText="保存"
        cancelText="取消"
        onOk={submit}
        onCancel={() => setOpen(false)}
        destroyOnHidden
      >
        <Form form={form} name="plotArc" layout="vertical" preserve={false}>
          <Form.Item name="type" label="类型">
            <Select options={ARC_TYPES.map((t) => ({ value: t, label: PLOT_ARC_LABELS[t] }))} />
          </Form.Item>
          <Form.Item name="title" label="标题" rules={[{ required: true, message: '请输入标题' }]}>
            <Input placeholder="例如：老城改造之争" maxLength={200} />
          </Form.Item>
          <Form.Item name="summary" label="摘要">
            <Input.TextArea rows={3} />
          </Form.Item>
          <Form.Item name="start_event_id" label="起始事件">
            <Select allowClear options={eventOptions} showSearch optionFilterProp="label" placeholder="（可选）" />
          </Form.Item>
          <Form.Item name="end_event_id" label="结束事件">
            <Select allowClear options={eventOptions} showSearch optionFilterProp="label" placeholder="（可选）" />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
