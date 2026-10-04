import { errorMessage } from '../../api/client'
import { ArrowDownOutlined, ArrowUpOutlined, DeleteOutlined, PlusOutlined, ReloadOutlined, SaveOutlined } from '@ant-design/icons'
import {
  Button,
  Card,
  Empty,
  Form,
  Input,
  InputNumber,
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

import { characterApi } from '../../api/characters'
import { eventApi } from '../../api/events'
import { worldApi } from '../../api/world'
import type {
  Character,
  EventInput,
  OriginalEvent,
  TimelineItem,
  WorldLocation,
} from '../../api/types'
import { useOriginalStore } from '../../stores/originalStore'

/** 时间线编排行（本地状态，点保存才落库） */
interface Row {
  event: OriginalEvent
  time_label: string
  duration: string
}

export default function OriginalTimelinePage() {
  const { workId, work, loadWork } = useOriginalStore()
  const [events, setEvents] = useState<OriginalEvent[]>([])
  const [rows, setRows] = useState<Row[]>([])
  const [characters, setCharacters] = useState<Character[]>([])
  const [locations, setLocations] = useState<WorldLocation[]>([])
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<OriginalEvent | null>(null)
  const [form] = Form.useForm<EventInput & { chapter_no?: number | null }>()

  const reload = useCallback(async () => {
    if (!workId) return
    setLoading(true)
    try {
      const [eventList, timeline, chars, locs] = await Promise.all([
        eventApi.list(workId, { pageSize: 500 }),
        eventApi.getTimeline(workId),
        characterApi.list(workId, { pageSize: 200 }),
        worldApi.listLocations(workId),
      ])
      setEvents(eventList.items)
      setCharacters(chars.items)
      setLocations(locs.items)
      setRows(
        timeline.entries.map((e) => ({
          event: e.event,
          time_label: e.time_label,
          duration: e.duration,
        })),
      )
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
    form.setFieldsValue({ importance: 3, time_order: 0, participants: [] })
    setOpen(true)
  }

  const openEdit = (record: OriginalEvent) => {
    setEditing(record)
    form.setFieldsValue({
      title: record.title,
      description: record.description,
      chapter_no: record.chapter_no,
      time_order: record.time_order,
      participants: record.participants,
      location_id: record.location_id,
      location_text: record.location_text,
      consequences: record.consequences,
      importance: record.importance,
    })
    setOpen(true)
  }

  const submitEvent = async () => {
    if (!workId) return
    const values = await form.validateFields()
    try {
      if (editing) await eventApi.update(editing.id, values)
      else await eventApi.create(workId, values)
      message.success(editing ? '事件已更新' : '事件已新增')
      setOpen(false)
      await reload()
    } catch (err) {
      message.error(errorMessage(err))
    }
  }

  const removeEvent = async (record: OriginalEvent) => {
    try {
      await eventApi.remove(record.id)
      message.success('事件已删除')
      await reload()
    } catch (err) {
      message.error(errorMessage(err))
    }
  }

  const inTimeline = (id: string) => rows.some((r) => r.event.id === id)

  const addToTimeline = (event: OriginalEvent) => {
    if (inTimeline(event.id)) {
      message.info('该事件已在时间线上')
      return
    }
    setRows([...rows, { event, time_label: '', duration: '' }])
  }

  const move = (index: number, delta: number) => {
    const next = [...rows]
    const target = index + delta
    if (target < 0 || target >= next.length) return
    ;[next[index], next[target]] = [next[target], next[index]]
    setRows(next)
  }

  const setLabel = (index: number, field: 'time_label' | 'duration', value: string) => {
    const next = [...rows]
    next[index] = { ...next[index], [field]: value }
    setRows(next)
  }

  const saveTimeline = async () => {
    if (!workId) return
    setSaving(true)
    try {
      const items: TimelineItem[] = rows.map((r) => ({
        event_id: r.event.id,
        time_label: r.time_label,
        duration: r.duration,
      }))
      await eventApi.saveTimeline(workId, items)
      message.success(`时间线已保存（${items.length} 个事件）`)
      await reload()
    } catch (err) {
      message.error(errorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  if (!workId) {
    return (
      <Card title="时间线">
        <Empty description={<span>还没有选择原著，<Link to="/original/overview">去选择</Link></span>} />
      </Card>
    )
  }

  const eventColumns: ColumnsType<OriginalEvent> = [
    { title: '事件', dataIndex: 'title', key: 'title' },
    {
      title: '章节',
      dataIndex: 'chapter_no',
      key: 'chapter_no',
      width: 90,
      render: (v: number | null) => (v ? `第 ${v} 章` : '—'),
    },
    { title: '排序号', dataIndex: 'time_order', key: 'time_order', width: 100 },
    {
      title: '重要度',
      dataIndex: 'importance',
      key: 'importance',
      width: 100,
      render: (v: number) => <Tag color={v >= 4 ? 'red' : v === 3 ? 'blue' : 'default'}>{v}</Tag>,
    },
    {
      title: '参与者',
      dataIndex: 'participants',
      key: 'participants',
      width: 200,
      render: (ids: string[]) =>
        ids.length === 0 ? (
          <Typography.Text type="secondary">—</Typography.Text>
        ) : (
          <Space size={4} wrap>
            {ids.map((id) => (
              <Tag key={id}>{characters.find((c) => c.id === id)?.name ?? id.slice(0, 6)}</Tag>
            ))}
          </Space>
        ),
    },
    {
      title: '操作',
      key: 'actions',
      width: 230,
      render: (_, record) => (
        <Space>
          <Button type="link" size="small" disabled={inTimeline(record.id)} onClick={() => addToTimeline(record)}>
            加入时间线
          </Button>
          <Button type="link" size="small" onClick={() => openEdit(record)}>
            编辑
          </Button>
          <Popconfirm title="删除该事件？" okText="确认删除" cancelText="取消" onConfirm={() => removeEvent(record)}>
            <Button type="link" size="small" danger>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
      <Card
        title={work ? `《${work.title}》事件库（${events.length}）` : '事件库'}
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={() => void reload()}>
              刷新
            </Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              新增事件
            </Button>
          </Space>
        }
      >
        {events.length === 0 && !loading ? (
          <Empty description="还没有事件，先把原著里发生的关键事件记下来" />
        ) : (
          <Table<OriginalEvent>
            rowKey="id"
            loading={loading}
            dataSource={events}
            columns={eventColumns}
            pagination={{ pageSize: 20 }}
          />
        )}
      </Card>

      <Card
        title={`时间线（${rows.length} 个事件）`}
        extra={
          <Button type="primary" icon={<SaveOutlined />} loading={saving} onClick={saveTimeline} disabled={rows.length === 0}>
            保存时间线
          </Button>
        }
      >
        {rows.length === 0 ? (
          <Empty description="从上面的「事件库」点「加入时间线」开始编排顺序" />
        ) : (
          <Space direction="vertical" size="small" style={{ display: 'flex' }}>
            {rows.map((row, index) => (
              <Space key={row.event.id} align="center" style={{ display: 'flex' }}>
                <Tag color="blue" style={{ width: 40, textAlign: 'center' }}>
                  {index + 1}
                </Tag>
                <Typography.Text style={{ width: 220 }} ellipsis={{ tooltip: row.event.title }}>
                  {row.event.title}
                </Typography.Text>
                <Input
                  placeholder="时间标签（如：三年前·梅雨季）"
                  style={{ width: 220 }}
                  value={row.time_label}
                  onChange={(e) => setLabel(index, 'time_label', e.target.value)}
                />
                <Input
                  placeholder="持续（如：两天）"
                  style={{ width: 140 }}
                  value={row.duration}
                  onChange={(e) => setLabel(index, 'duration', e.target.value)}
                />
                <Button
                  size="small"
                  icon={<ArrowUpOutlined />}
                  aria-label={`上移：${row.event.title}`}
                  disabled={index === 0}
                  onClick={() => move(index, -1)}
                />
                <Button
                  size="small"
                  icon={<ArrowDownOutlined />}
                  aria-label={`下移：${row.event.title}`}
                  disabled={index === rows.length - 1}
                  onClick={() => move(index, 1)}
                />
                <Button
                  size="small"
                  danger
                  icon={<DeleteOutlined />}
                  aria-label={`移出时间线：${row.event.title}`}
                  onClick={() => setRows(rows.filter((_, i) => i !== index))}
                />
              </Space>
            ))}
          </Space>
        )}
      </Card>

      <Modal
        open={open}
        title={editing ? `编辑事件：${editing.title}` : '新增事件'}
        width={720}
        okText="保存"
        cancelText="取消"
        onOk={submitEvent}
        onCancel={() => setOpen(false)}
        destroyOnHidden
      >
        <Form form={form} name="event" layout="vertical" preserve={false}>
          <Form.Item name="title" label="事件标题" rules={[{ required: true, message: '请输入事件标题' }]}>
            <Input placeholder="例如：母亲的意外" maxLength={200} />
          </Form.Item>
          <Space size="middle" style={{ display: 'flex' }}>
            <Form.Item name="chapter_no" label="章节号" style={{ minWidth: 140 }}>
              <InputNumber min={1} placeholder="可空" style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="time_order" label="排序号" style={{ minWidth: 140 }} extra="越小越靠前">
              <InputNumber style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="importance" label="重要度" style={{ minWidth: 160 }}>
              <Select options={[5, 4, 3, 2, 1].map((v) => ({ value: v, label: `${v}` }))} />
            </Form.Item>
          </Space>
          <Form.Item name="participants" label="参与者">
            <Select
              mode="multiple"
              allowClear
              placeholder="选择参与的人物"
              options={characters.map((c) => ({ value: c.id, label: c.name }))}
              optionFilterProp="label"
            />
          </Form.Item>
          <Form.Item name="location_id" label="发生地点">
            <Select
              allowClear
              placeholder="（可选，来自世界观的地点）"
              options={locations.map((l) => ({ value: l.id, label: l.name }))}
              optionFilterProp="label"
            />
          </Form.Item>
          <Form.Item name="location_text" label="地点补充说明">
            <Input placeholder="例如：老糖厂三号车间" maxLength={200} />
          </Form.Item>
          <Form.Item name="description" label="事件描述">
            <Input.TextArea rows={3} />
          </Form.Item>
          <Form.Item name="consequences" label="后果 / 影响">
            <Input.TextArea rows={2} placeholder="这件事之后，什么被改变了？" />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
