import { errorMessage } from '../api/client'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import {
  Alert,
  Button,
  Card,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Slider,
  Space,
  Statistic,
  Table,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'

import { characterApi } from '../api/characters'
import { creativeApi, createCreativeWork } from '../api/creative'
import { eventApi } from '../api/events'
import { projectApi } from '../api/projects'
import EntityVersions from '../components/EntityVersions'
import {
  CREATIVE_RULE_STATUS_LABEL,
  CREATIVE_SOURCE_LABEL,
  DNA_KEYS,
  DNA_LABELS,
  MAPPING_LABEL,
  WORLD_MODE_LABEL,
  type Character,
  type CreativeCharacter,
  type CreativeMapping,
  type CreativeRuleStatus,
  type CreativeTimelineEvent,
  type CreativeWork,
  type CreativeWorld,
  type DNAKey,
  type DivergencePoint,
  type OriginalEvent,
  type Project,
  type WorldInheritanceMode,
} from '../api/types'
import { useOriginalStore } from '../stores/originalStore'

const WEIGHT_KEYS: Array<{ key: DNAKey; field: string }> = [
  { key: 'personality', field: 'personality' },
  { key: 'values', field: 'values' },
  { key: 'motivation', field: 'motivation' },
  { key: 'behavior', field: 'behavior' },
  { key: 'speech_style', field: 'speech_style' },
  { key: 'background', field: 'background' },
  { key: 'ability', field: 'ability' },
  { key: 'relationship_pattern', field: 'relationship_pattern' },
]

interface Props {
  /** 从左侧菜单进入时直接落到对应标签 */
  defaultTab?: 'characters' | 'world' | 'timeline' | 'mappings'
}

export default function CreativePage({ defaultTab = 'characters' }: Props) {
  const { workId: originalWorkId, work: originalWork, loadWork } = useOriginalStore()

  const [works, setWorks] = useState<CreativeWork[]>([])
  const [work, setWork] = useState<CreativeWork | null>(null)
  const [characters, setCharacters] = useState<CreativeCharacter[]>([])
  const [sourceCharacters, setSourceCharacters] = useState<Character[]>([])
  const [world, setWorld] = useState<CreativeWorld | null>(null)
  const [divergence, setDivergence] = useState<DivergencePoint | null>(null)
  const [timeline, setTimeline] = useState<CreativeTimelineEvent[]>([])
  const [mappings, setMappings] = useState<CreativeMapping[]>([])
  const [events, setEvents] = useState<OriginalEvent[]>([])
  const [loading, setLoading] = useState(false)
  const [activeTab, setActiveTab] = useState(defaultTab)

  const [createOpen, setCreateOpen] = useState(false)
  const [createForm] = Form.useForm<{ project_id: string; title: string; description?: string }>()
  const [creativeProjects, setCreativeProjects] = useState<Project[]>([])

  const [inheritOpen, setInheritOpen] = useState(false)
  const [inheritForm] = Form.useForm<{ source_character_id: string; name?: string; importance?: number }>()
  const [inheritWeights, setInheritWeights] = useState<Record<string, number>>(
    Object.fromEntries(WEIGHT_KEYS.map((w) => [w.field, 100])),
  )

  const [fuseOpen, setFuseOpen] = useState(false)
  const [fuseForm] = Form.useForm<{ name: string; importance?: number }>()
  const [fuseSources, setFuseSources] = useState<Array<{ character_id?: string; weight?: number }>>([
    { weight: 60 },
    { weight: 40 },
  ])

  const loadAll = useCallback(async (target?: CreativeWork) => {
    if (!originalWorkId) return
    setLoading(true)
    try {
      const list = await creativeApi.listByOriginal(originalWorkId)
      setWorks(list.items)
      const current = target ?? list.items[0] ?? null
      setWork(current)
      if (!current) {
        setCharacters([])
        setWorld(null)
        setTimeline([])
        setMappings([])
        setLoading(false)
        return
      }
      const [chars, w, tl, mp, dp, srcChars, evts] = await Promise.all([
        creativeApi.listCharacters(current.id),
        creativeApi.getWorld(current.id),
        creativeApi.getTimeline(current.id),
        creativeApi.listMappings(current.id),
        creativeApi.getDivergence(current.id),
        characterApi.list(originalWorkId, { pageSize: 200 }),
        eventApi.list(originalWorkId, { pageSize: 200 }),
      ])
      setCharacters(chars.items)
      setWorld(w)
      setTimeline(tl.items)
      setMappings(mp.items)
      setDivergence(dp)
      setSourceCharacters(srcChars.items)
      setEvents(evts.items)
    } catch (err) {
      message.error(errorMessage(err))
    } finally {
      setLoading(false)
    }
  }, [originalWorkId])

  useEffect(() => {
    if (!originalWorkId) return
    if (!originalWork) void loadWork(originalWorkId).catch((err: Error) => message.error(err.message))
  }, [originalWorkId, originalWork, loadWork])

  useEffect(() => {
    void loadAll()
  }, [loadAll])

  /** 继承预览：按滑杆权重实时算出会派生出的 DNA */
  const inheritancePreview = (sourceId?: string) => {
    const source = sourceCharacters.find((c) => c.id === sourceId)
    if (!source) return []
    return WEIGHT_KEYS.map(({ key, field }) => {
      const dim = source.dna?.[key]
      const weight = inheritWeights[field] ?? 0
      return {
        key,
        label: DNA_LABELS[key],
        text: dim?.text ?? '',
        from: dim?.weight ?? 0,
        to: Math.floor(((dim?.weight ?? 0) * weight) / 100),
      }
    }).filter((d) => d.to > 0)
  }

  const createWork = async () => {
    if (!originalWorkId) return
    const values = await createForm.validateFields()
    try {
      const created = await createCreativeWork(originalWorkId, values)
      message.success('二创作品已创建')
      setCreateOpen(false)
      await loadAll(created)
    } catch (err) {
      message.error(errorMessage(err))
    }
  }

  const openCreate = async () => {
    try {
      const list = await projectApi.list({ type: 'CREATIVE', page_size: 100 })
      setCreativeProjects(list.items)
      createForm.resetFields()
      if (originalWork) createForm.setFieldsValue({ title: `${originalWork.title}·二创` })
      setCreateOpen(true)
    } catch (err) {
      message.error(errorMessage(err))
    }
  }

  const submitInherit = async () => {
    if (!work) return
    const values = await inheritForm.validateFields()
    try {
      const created = await creativeApi.inheritCharacter(work.id, {
        source_character_id: values.source_character_id,
        name: values.name,
        importance: values.importance,
        weights: inheritWeights,
      })
      message.success(`已继承：${created.name}`)
      setInheritOpen(false)
      await loadAll(work)
    } catch (err) {
      message.error(errorMessage(err))
    }
  }

  const submitFuse = async () => {
    if (!work) return
    const values = await fuseForm.validateFields()
    const sources = fuseSources
      .filter((s) => s.character_id)
      .map((s) => ({ character_id: s.character_id as string, weight: s.weight ?? 50 }))
    if (sources.length < 2) {
      message.warning('至少选择两个融合来源')
      return
    }
    try {
      const created = await creativeApi.fuseCharacters(work.id, { ...values, sources })
      message.success(`已融合：${created.name}`)
      setFuseOpen(false)
      await loadAll(work)
    } catch (err) {
      message.error(errorMessage(err))
    }
  }

  const characterColumns: ColumnsType<CreativeCharacter> = [
    {
      title: '人物',
      key: 'name',
      render: (_, r) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{r.name}</Typography.Text>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {CREATIVE_SOURCE_LABEL[r.source_type]}
            {r.is_locked ? ' · 已锁定' : ''}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: 'DNA',
      key: 'dna',
      render: (_, r) => {
        const dims = DNA_KEYS.map((k) => ({ k, ...(r.dna?.[k] ?? { weight: 0 }) }))
          .filter((d) => (d.weight ?? 0) > 0)
          .sort((a, b) => (b.weight ?? 0) - (a.weight ?? 0))
          .slice(0, 4)
        if (dims.length === 0) return <Typography.Text type="secondary">未设置</Typography.Text>
        return (
          <Space wrap size={4}>
            {dims.map((d) => (
              <Tag key={d.k} color="purple">
                {DNA_LABELS[d.k]} {d.weight}%
              </Tag>
            ))}
          </Space>
        )
      },
    },
    {
      title: '融合来源',
      key: 'fusion',
      width: 220,
      render: (_, r) =>
        r.fusion_sources?.length ? (
          <Space wrap size={4}>
            {r.fusion_sources.map((s) => (
              <Tag key={s.character_id}>
                {s.name} {s.weight}%
              </Tag>
            ))}
          </Space>
        ) : (
          <Typography.Text type="secondary">—</Typography.Text>
        ),
    },
    {
      title: '操作',
      key: 'actions',
      width: 220,
      render: (_, r) => (
        <Space>
          <Button
            type="link"
            size="small"
            onClick={async () => {
              try {
                await creativeApi.updateCharacter(r.id, { is_locked: !r.is_locked })
                await loadAll(work ?? undefined)
              } catch (err) {
                message.error(errorMessage(err))
              }
            }}
          >
            {r.is_locked ? '解锁' : '锁定'}
          </Button>
          <EntityVersions
            entityType="creative_character"
            entityId={r.id}
            onRestored={() => void loadAll(work ?? undefined)}
          />
          <Popconfirm
            title={`删除「${r.name}」？`}
            okText="确认删除"
            cancelText="取消"
            onConfirm={async () => {
              try {
                await creativeApi.deleteCharacter(r.id)
                await loadAll(work ?? undefined)
              } catch (err) {
                message.error(errorMessage(err))
              }
            }}
          >
            <Button type="link" size="small" danger>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  const ruleColumns: ColumnsType<CreativeWorld['rules'][number]> = [
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (v: CreativeRuleStatus) => (
        <Tag color={v === 'REMOVED' ? 'default' : v === 'NEW' ? 'purple' : v === 'MODIFIED' ? 'orange' : 'blue'}>
          {CREATIVE_RULE_STATUS_LABEL[v]}
        </Tag>
      ),
    },
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '分类', dataIndex: 'category', key: 'category', width: 140 },
    { title: '重要度', dataIndex: 'importance', key: 'importance', width: 90 },
    { title: '说明', dataIndex: 'description', key: 'description', ellipsis: true },
    {
      title: '操作',
      key: 'actions',
      width: 140,
      render: (_, r) => (
        <Space>
          <Button
            type="link"
            size="small"
            disabled={r.status === 'REMOVED'}
            onClick={async () => {
              let edited = r.description
              Modal.confirm({
                title: `修改规则：${r.name}`,
                content: (
                  <Input.TextArea
                    defaultValue={r.description}
                    rows={3}
                    onChange={(e) => {
                      edited = e.target.value
                    }}
                  />
                ),
                okText: '保存',
                cancelText: '取消',
                onOk: async () => {
                  try {
                    await creativeApi.updateWorldRule(r.id, {
                      name: r.name,
                      category: r.category,
                      description: edited,
                      importance: r.importance,
                    })
                    await loadAll(work ?? undefined)
                    message.success('已修改（状态变为「已修改」）')
                  } catch (err) {
                    message.error(errorMessage(err))
                  }
                },
              })
            }}
          >
            修改
          </Button>
          <Popconfirm
            title="在二创里删掉这条规则？"
            description="继承来的规则会标记为「已删除」并保留可追溯性"
            okText="确认删除"
            cancelText="取消"
            onConfirm={async () => {
              try {
                await creativeApi.deleteWorldRule(r.id)
                await loadAll(work ?? undefined)
              } catch (err) {
                message.error(errorMessage(err))
              }
            }}
          >
            <Button type="link" size="small" danger disabled={r.status === 'REMOVED'}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  const timelineColumns: ColumnsType<CreativeTimelineEvent> = [
    { title: '#', dataIndex: 'sequence', key: 'sequence', width: 60 },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (v: string) => <Tag color={v === 'INHERITED' ? 'blue' : v === 'NEW' ? 'purple' : 'default'}>{v}</Tag>,
    },
    { title: '标题', dataIndex: 'title', key: 'title' },
    { title: '时间标签', dataIndex: 'time_label', key: 'time_label', width: 180 },
  ]

  const mappingColumns: ColumnsType<CreativeMapping> = [
    {
      title: '类型',
      dataIndex: 'mapping_type',
      key: 'mapping_type',
      width: 100,
      render: (v: keyof typeof MAPPING_LABEL) => <Tag>{MAPPING_LABEL[v] ?? v}</Tag>,
    },
    { title: '原著对象', dataIndex: 'original_type', key: 'original_type', width: 140 },
    { title: '二创对象', dataIndex: 'creative_type', key: 'creative_type', width: 200 },
    { title: '说明', dataIndex: 'description', key: 'description', ellipsis: true },
  ]

  if (!originalWorkId) {
    return (
      <Card title="二创工作区">
        <Empty
          description={<span>先在「原著 · 总览」里选一部原著，再回到这里创建二创作品</span>}
        />
      </Card>
    )
  }

  return (
    <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
      <Card
        title={work ? `二创：${work.title}` : '二创工作区'}
        extra={
          <Space>
            {works.length > 1 && (
              <Select
                style={{ width: 240 }}
                value={work?.id}
                onChange={(id) => void loadAll(works.find((w) => w.id === id))}
                options={works.map((w) => ({ value: w.id, label: w.title }))}
              />
            )}
            <Button icon={<ReloadOutlined />} onClick={() => void loadAll(work ?? undefined)}>
              刷新
            </Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              新建二创作品
            </Button>
          </Space>
        }
      >
        {work ? (
          <Space size="large">
            <Statistic title="二创人物" value={characters.length} />
            <Statistic title="世界规则" value={world?.rules.length ?? 0} />
            <Statistic title="时间线事件" value={timeline.length} />
            <Statistic title="映射关系" value={mappings.length} />
            <Statistic title="分叉点" value={divergence ? '已设' : '未设'} />
          </Space>
        ) : (
          <Empty description="还没有二创作品：选一个 CREATIVE 工程，从当前原著创建一部二创" />
        )}
      </Card>

      {work && (
        <Card>
          <Tabs
            activeKey={activeTab}
            onChange={(key) => setActiveTab(key as typeof activeTab)}
            items={[
              {
                key: 'characters',
                label: `人物（${characters.length}）`,
                children: (
                  <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
                    <Alert
                      type="info"
                      showIcon
                      message="继承是把原著人物的 DNA 按你给的权重带过来"
                      description="权重 0 表示完全不带、100 表示原样保留。派生结果只是起点，之后你可以继续改。"
                    />
                    <Space>
                      <Button
                        type="primary"
                        onClick={() => {
                          inheritForm.resetFields()
                          setInheritWeights(Object.fromEntries(WEIGHT_KEYS.map((w) => [w.field, 100])))
                          setInheritOpen(true)
                        }}
                      >
                        从原著继承人物
                      </Button>
                      <Button
                        onClick={() => {
                          fuseForm.resetFields()
                          setFuseSources([{ weight: 60 }, { weight: 40 }])
                          setFuseOpen(true)
                        }}
                        disabled={characters.length < 2}
                      >
                        人物融合
                      </Button>
                    </Space>
                    <Table<CreativeCharacter>
                      rowKey="id"
                      loading={loading}
                      dataSource={characters}
                      columns={characterColumns}
                      pagination={false}
                    />
                  </Space>
                ),
              },
              {
                key: 'world',
                label: `世界（${world?.rules.length ?? 0}）`,
                children: (
                  <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
                    <Space wrap>
                      {(Object.keys(WORLD_MODE_LABEL) as WorldInheritanceMode[]).map((mode) => (
                        <Popconfirm
                          key={mode}
                          title={`按「${WORLD_MODE_LABEL[mode]}」继承原著世界？`}
                          description="FULL/PARTIAL 会把原著规则整套带过来；MODIFIED/NEW 先建空世界"
                          okText="继承"
                          cancelText="取消"
                          onConfirm={async () => {
                            try {
                              await creativeApi.inheritWorld(work.id, mode)
                              message.success(`已按 ${WORLD_MODE_LABEL[mode]} 建立二创世界`)
                              await loadAll(work)
                            } catch (err) {
                              message.error(errorMessage(err))
                            }
                          }}
                        >
                          <Button>{WORLD_MODE_LABEL[mode]}</Button>
                        </Popconfirm>
                      ))}
                      <Typography.Text type="secondary">
                        当前模式：{world?.inheritance_mode ? WORLD_MODE_LABEL[world.inheritance_mode] : '未建世界'}
                      </Typography.Text>
                      {world?.id && work && (
                        <EntityVersions
                          entityType="creative_world"
                          entityId={work.id}
                          onRestored={() => void loadAll(work)}
                        />
                      )}
                    </Space>
                    {world?.id && (
                      <>
                        <Space>
                          <Tag color="blue">继承 {world.inherited_count}</Tag>
                          <Tag color="orange">已改 {world.modified_count}</Tag>
                          <Tag>已删 {world.removed_count}</Tag>
                          <Tag color="purple">新增 {world.new_count}</Tag>
                        </Space>
                        <Table
                          rowKey="id"
                          dataSource={world.rules}
                          columns={ruleColumns}
                          pagination={false}
                        />
                      </>
                    )}
                  </Space>
                ),
              },
              {
                key: 'timeline',
                label: `时间线（${timeline.length}）`,
                children: (
                  <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
                    <Space wrap>
                      <Select
                        style={{ width: 300 }}
                        placeholder="选择分叉点（原著事件）"
                        value={divergence?.original_event_id ?? undefined}
                        onChange={async (value) => {
                          try {
                            const point = await creativeApi.setDivergence(work.id, { original_event_id: value })
                            setDivergence(point)
                            message.success('分叉点已设置')
                          } catch (err) {
                            message.error(errorMessage(err))
                          }
                        }}
                        options={events.map((e) => ({ value: e.id, label: e.title }))}
                      />
                      <Button
                        type="primary"
                        onClick={async () => {
                          try {
                            const built = await creativeApi.buildTimeline(work.id)
                            setTimeline(built.items)
                            message.success(`已构建时间线：${built.items.length} 条`)
                            await loadAll(work)
                          } catch (err) {
                            message.error(errorMessage(err))
                          }
                        }}
                      >
                        按分叉点自动构建
                      </Button>
                    </Space>
                    <Alert
                      type="warning"
                      showIcon
                      message="分叉点之前继承原著事件，之后由你写"
                      description="重新构建不会覆盖你手工添加的二创事件（它们会保留在末尾）。"
                    />
                    {timeline.length === 0 ? (
                      <Empty description="还没有时间线：先设分叉点，再点「按分叉点自动构建」" />
                    ) : (
                      <Table
                        rowKey="id"
                        dataSource={timeline}
                        columns={timelineColumns}
                        pagination={false}
                      />
                    )}
                  </Space>
                ),
              },
              {
                key: 'mappings',
                label: `映射（${mappings.length}）`,
                children: (
                  <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
                    <Typography.Text type="secondary">
                      每个二创元素来自哪条原著设定，都在这里；「删除」表示原著有、但二创里被作者去掉了。
                    </Typography.Text>
                    <Table rowKey="id" dataSource={mappings} columns={mappingColumns} pagination={{ pageSize: 20 }} />
                  </Space>
                ),
              },
            ]}
          />
        </Card>
      )}

      <Modal
        open={createOpen}
        title="新建二创作品"
        okText="创建"
        cancelText="取消"
        onOk={createWork}
        onCancel={() => setCreateOpen(false)}
        destroyOnHidden
      >
        <Form form={createForm} name="createCreative" layout="vertical" preserve={false}>
          <Form.Item
            name="project_id"
            label="二创工程"
            rules={[{ required: true, message: '请选择二创工程' }]}
            extra="必须是 CREATIVE 类型的工程；到「工程管理」新建"
          >
            <Select
              options={creativeProjects.map((p) => ({ value: p.id, label: p.name }))}
              notFoundContent={<Link to="/projects">去工程管理创建</Link>}
            />
          </Form.Item>
          <Form.Item name="title" label="作品标题" rules={[{ required: true, message: '请输入标题' }]}>
            <Input maxLength={200} />
          </Form.Item>
          <Form.Item name="description" label="一句话设定">
            <Input.TextArea rows={2} placeholder="例如：如果那天林默没有离开" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        open={inheritOpen}
        title="从原著继承人物"
        width={700}
        okText="继承"
        cancelText="取消"
        onOk={submitInherit}
        onCancel={() => setInheritOpen(false)}
        destroyOnHidden
      >
        <Form form={inheritForm} name="inheritCharacter" layout="vertical" preserve={false}>
          <Space size="middle" style={{ display: 'flex' }}>
            <Form.Item
              name="source_character_id"
              label="原著人物"
              rules={[{ required: true, message: '请选择原著人物' }]}
              style={{ minWidth: 260 }}
            >
              <Select options={sourceCharacters.map((c) => ({ value: c.id, label: c.name }))} showSearch optionFilterProp="label" />
            </Form.Item>
            <Form.Item name="name" label="二创姓名（留空沿用）" style={{ minWidth: 200 }}>
              <Input maxLength={120} />
            </Form.Item>
            <Form.Item name="importance" label="重要度" style={{ minWidth: 120 }}>
              <InputNumber min={1} max={5} defaultValue={3} />
            </Form.Item>
          </Space>
        </Form>
        <Typography.Text strong>各维度继承权重</Typography.Text>
        {WEIGHT_KEYS.map(({ key, field }) => (
          <Space key={field} align="baseline" style={{ display: 'flex' }}>
            <Typography.Text style={{ width: 84, display: 'inline-block' }}>{DNA_LABELS[key]}</Typography.Text>
            <Slider
              style={{ width: 360 }}
              min={0}
              max={100}
              step={5}
              value={inheritWeights[field]}
              onChange={(v) => setInheritWeights((prev) => ({ ...prev, [field]: v }))}
            />
            <Typography.Text type="secondary">{inheritWeights[field]}%</Typography.Text>
          </Space>
        ))}
        <Form.Item noStyle shouldUpdate>
          {() => {
            const sourceId = inheritForm.getFieldValue('source_character_id') as string | undefined
            const rows = inheritancePreview(sourceId)
            if (rows.length === 0) return null
            return (
              <Alert
                style={{ marginTop: 12 }}
                type="success"
                message="派生预览（原著权重 → 继承后权重）"
                description={
                  <Space wrap size={4}>
                    {rows.map((r) => (
                      <Tag key={r.key}>
                        {r.label} {r.from} → <b>{r.to}</b>
                      </Tag>
                    ))}
                  </Space>
                }
              />
            )
          }}
        </Form.Item>
      </Modal>

      <Modal
        open={fuseOpen}
        title="人物融合"
        okText="融合"
        cancelText="取消"
        onOk={submitFuse}
        onCancel={() => setFuseOpen(false)}
        destroyOnHidden
      >
        <Form form={fuseForm} name="fuseCharacter" layout="vertical" preserve={false}>
          <Form.Item name="name" label="新人物姓名" rules={[{ required: true, message: '请输入姓名' }]}>
            <Input maxLength={120} />
          </Form.Item>
          <Form.Item name="importance" label="重要度">
            <InputNumber min={1} max={5} defaultValue={3} />
          </Form.Item>
        </Form>
        <Typography.Text strong>融合来源（整体权重决定该来源的影响力）</Typography.Text>
        {fuseSources.map((source, index) => (
          <Space key={index} style={{ display: 'flex', marginTop: 8 }}>
            <Select
              style={{ width: 200 }}
              placeholder="选择一个二创人物"
              value={source.character_id}
              onChange={(v) => {
                const next = [...fuseSources]
                next[index] = { ...next[index], character_id: v }
                setFuseSources(next)
              }}
              options={characters.map((c) => ({ value: c.id, label: c.name }))}
            />
            <Slider
              style={{ width: 220 }}
              min={0}
              max={100}
              step={5}
              value={source.weight}
              onChange={(v) => {
                const next = [...fuseSources]
                next[index] = { ...next[index], weight: v }
                setFuseSources(next)
              }}
            />
            <Typography.Text type="secondary">{source.weight}%</Typography.Text>
          </Space>
        ))}
        <Button
          style={{ marginTop: 8 }}
          onClick={() => setFuseSources([...fuseSources, { weight: 50 }])}
          icon={<PlusOutlined />}
        >
          再加一个来源
        </Button>
      </Modal>
    </Space>
  )
}
