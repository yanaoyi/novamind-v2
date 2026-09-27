import { PlusOutlined, ReloadOutlined, SaveOutlined } from '@ant-design/icons'
import {
  Button,
  Card,
  Descriptions,
  Empty,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Slider,
  Space,
  Tabs,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'

import { worldApi } from '../../api/world'
import type { World, WorldLocation, WorldRule, Faction } from '../../api/types'
import { useOriginalStore } from '../../stores/originalStore'

interface Props {
  /** 从左侧菜单进来时直接落到对应标签页 */
  defaultTab?: 'rules' | 'locations' | 'factions'
}

export default function OriginalWorldPage({ defaultTab = 'rules' }: Props) {
  const { workId, work, loadWork } = useOriginalStore()

  const [world, setWorld] = useState<World | null>(null)
  const [rules, setRules] = useState<WorldRule[]>([])
  const [locations, setLocations] = useState<WorldLocation[]>([])
  const [factions, setFactions] = useState<Faction[]>([])
  const [loading, setLoading] = useState(false)

  const [worldForm] = Form.useForm<{ name?: string; description?: string }>()
  const [ruleForm] = Form.useForm<{ category?: string; name: string; description?: string; importance: number }>()
  const [locationForm] = Form.useForm<{ name: string; type?: string; description?: string; parent_location_id?: string | null }>()
  const [factionForm] = Form.useForm<{ name: string; type?: string; description?: string; goals?: string; relationships?: string }>()

  const [ruleOpen, setRuleOpen] = useState(false)
  const [locationOpen, setLocationOpen] = useState(false)
  const [factionOpen, setFactionOpen] = useState(false)
  const [editingRule, setEditingRule] = useState<WorldRule | null>(null)
  const [editingLocation, setEditingLocation] = useState<WorldLocation | null>(null)
  const [editingFaction, setEditingFaction] = useState<Faction | null>(null)

  const reload = useCallback(async () => {
    if (!workId) return
    setLoading(true)
    try {
      const [w, r, l, f] = await Promise.all([
        worldApi.get(workId),
        worldApi.listRules(workId),
        worldApi.listLocations(workId),
        worldApi.listFactions(workId),
      ])
      setWorld(w)
      setRules(r.items)
      setLocations(l.items)
      setFactions(f.items)
      worldForm.setFieldsValue({ name: w?.name ?? '', description: w?.description ?? '' })
    } catch (err) {
      message.error((err as Error).message)
    } finally {
      setLoading(false)
    }
  }, [workId, worldForm])

  useEffect(() => {
    if (!workId) return
    if (!work) void loadWork(workId).catch((err: Error) => message.error(err.message))
  }, [workId, work, loadWork])

  useEffect(() => {
    void reload()
  }, [reload])

  /** 地点完整路径（大陆 / 国家 / 城市） */
  const locationPath = useCallback(
    (id: string): string => {
      const names: string[] = []
      let current = locations.find((l) => l.id === id)
      let guard = 0
      while (current && guard < 20) {
        names.unshift(current.name)
        current = current.parent_location_id
          ? locations.find((l) => l.id === current?.parent_location_id)
          : undefined
        guard += 1
      }
      return names.join(' / ')
    },
    [locations],
  )

  /** 编辑时排除自己与自己的后代，避免用户提交出环（后端也会拦） */
  const forbiddenParents = useMemo(() => {
    if (!editingLocation) return new Set<string>()
    const bad = new Set<string>([editingLocation.id])
    let changed = true
    while (changed) {
      changed = false
      for (const loc of locations) {
        if (loc.parent_location_id && bad.has(loc.parent_location_id) && !bad.has(loc.id)) {
          bad.add(loc.id)
          changed = true
        }
      }
    }
    return bad
  }, [editingLocation, locations])

  const saveWorld = async () => {
    if (!workId) return
    const values = await worldForm.validateFields()
    try {
      await worldApi.save(workId, values)
      message.success('世界设定已保存')
      await reload()
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  const submitRule = async () => {
    if (!workId) return
    const values = await ruleForm.validateFields()
    try {
      if (editingRule) await worldApi.updateRule(editingRule.id, values)
      else await worldApi.createRule(workId, values)
      message.success(editingRule ? '规则已更新' : '规则已新增')
      setRuleOpen(false)
      await reload()
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  const submitLocation = async () => {
    if (!workId) return
    const values = await locationForm.validateFields()
    const payload = { ...values, parent_location_id: values.parent_location_id || null }
    try {
      if (editingLocation) await worldApi.updateLocation(editingLocation.id, payload)
      else await worldApi.createLocation(workId, payload)
      message.success(editingLocation ? '地点已更新' : '地点已新增')
      setLocationOpen(false)
      await reload()
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  const submitFaction = async () => {
    if (!workId) return
    const values = await factionForm.validateFields()
    try {
      if (editingFaction) await worldApi.updateFaction(editingFaction.id, values)
      else await worldApi.createFaction(workId, values)
      message.success(editingFaction ? '势力已更新' : '势力已新增')
      setFactionOpen(false)
      await reload()
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  if (!workId) {
    return (
      <Card title="世界观">
        <Empty description={<span>还没有选择原著，<Link to="/original/overview">去选择</Link></span>} />
      </Card>
    )
  }

  const ruleColumns: ColumnsType<WorldRule> = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '分类', dataIndex: 'category', key: 'category', width: 140 },
    {
      title: '重要度',
      dataIndex: 'importance',
      key: 'importance',
      width: 100,
      render: (v: number) => <Tag color={v >= 4 ? 'red' : v === 3 ? 'blue' : 'default'}>{v}</Tag>,
    },
    { title: '说明', dataIndex: 'description', key: 'description', ellipsis: true },
    {
      title: '操作',
      key: 'actions',
      width: 130,
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            size="small"
            onClick={() => {
              setEditingRule(record)
              ruleForm.setFieldsValue(record)
              setRuleOpen(true)
            }}
          >
            编辑
          </Button>
          <Popconfirm
            title="删除该规则？"
            okText="确认删除"
            cancelText="取消"
            onConfirm={async () => {
              try {
                await worldApi.removeRule(record.id)
                await reload()
              } catch (err) {
                message.error((err as Error).message)
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

  const locationColumns: ColumnsType<WorldLocation> = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '类型', dataIndex: 'type', key: 'type', width: 120 },
    {
      title: '层级',
      key: 'path',
      width: 260,
      render: (_, record) =>
        record.parent_location_id ? (
          <Typography.Text type="secondary">{locationPath(record.parent_location_id)}</Typography.Text>
        ) : (
          <Tag>顶层</Tag>
        ),
    },
    { title: '说明', dataIndex: 'description', key: 'description', ellipsis: true },
    {
      title: '操作',
      key: 'actions',
      width: 130,
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            size="small"
            onClick={() => {
              setEditingLocation(record)
              locationForm.setFieldsValue({ ...record, parent_location_id: record.parent_location_id ?? undefined })
              setLocationOpen(true)
            }}
          >
            编辑
          </Button>
          <Popconfirm
            title="删除该地点？子地点的上级会被置空"
            okText="确认删除"
            cancelText="取消"
            onConfirm={async () => {
              try {
                await worldApi.removeLocation(record.id)
                await reload()
              } catch (err) {
                message.error((err as Error).message)
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

  const factionColumns: ColumnsType<Faction> = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '类型', dataIndex: 'type', key: 'type', width: 120 },
    { title: '目标', dataIndex: 'goals', key: 'goals', ellipsis: true },
    { title: '关系', dataIndex: 'relationships', key: 'relationships', ellipsis: true },
    {
      title: '操作',
      key: 'actions',
      width: 130,
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            size="small"
            onClick={() => {
              setEditingFaction(record)
              factionForm.setFieldsValue(record)
              setFactionOpen(true)
            }}
          >
            编辑
          </Button>
          <Popconfirm
            title="删除该势力？"
            okText="确认删除"
            cancelText="取消"
            onConfirm={async () => {
              try {
                await worldApi.removeFaction(record.id)
                await reload()
              } catch (err) {
                message.error((err as Error).message)
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

  const locationOptions = locations
    .filter((l) => !forbiddenParents.has(l.id))
    .map((l) => ({ value: l.id, label: locationPath(l.id) }))

  return (
    <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
      <Card
        title={work ? `《${work.title}》世界观` : '世界观'}
        extra={
          <Button icon={<ReloadOutlined />} onClick={() => void reload()}>
            刷新
          </Button>
        }
      >
        <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
          <Form form={worldForm} name="world" layout="inline">
            <Form.Item name="name" label="世界名称" style={{ minWidth: 260 }}>
              <Input placeholder="例如：九州" maxLength={200} />
            </Form.Item>
            <Form.Item name="description" label="世界设定" style={{ minWidth: 420 }}>
              <Input placeholder="一句话说明这个世界的底层设定" />
            </Form.Item>
            <Form.Item>
              <Button type="primary" icon={<SaveOutlined />} onClick={saveWorld}>
                保存
              </Button>
            </Form.Item>
          </Form>

          <Descriptions column={3} size="small">
            <Descriptions.Item label="规则">{world?.rule_count ?? rules.length}</Descriptions.Item>
            <Descriptions.Item label="地点">{world?.location_count ?? locations.length}</Descriptions.Item>
            <Descriptions.Item label="势力">{world?.faction_count ?? factions.length}</Descriptions.Item>
          </Descriptions>
        </Space>
      </Card>

      <Card>
        <Tabs
          defaultActiveKey={defaultTab}
          items={[
            {
              key: 'rules',
              label: `世界规则（${rules.length}）`,
              children: (
                <>
                  <Button
                    type="primary"
                    icon={<PlusOutlined />}
                    style={{ marginBottom: 12 }}
                    onClick={() => {
                      setEditingRule(null)
                      ruleForm.resetFields()
                      ruleForm.setFieldsValue({ importance: 3 })
                      setRuleOpen(true)
                    }}
                  >
                    新增规则
                  </Button>
                  <Table<WorldRule>
                    rowKey="id"
                    loading={loading}
                    dataSource={rules}
                    columns={ruleColumns}
                    pagination={false}
                  />
                </>
              ),
            },
            {
              key: 'locations',
              label: `地点（${locations.length}）`,
              children: (
                <>
                  <Button
                    type="primary"
                    icon={<PlusOutlined />}
                    style={{ marginBottom: 12 }}
                    onClick={() => {
                      setEditingLocation(null)
                      locationForm.resetFields()
                      setLocationOpen(true)
                    }}
                  >
                    新增地点
                  </Button>
                  <Table<WorldLocation>
                    rowKey="id"
                    loading={loading}
                    dataSource={locations}
                    columns={locationColumns}
                    pagination={false}
                  />
                </>
              ),
            },
            {
              key: 'factions',
              label: `势力（${factions.length}）`,
              children: (
                <>
                  <Button
                    type="primary"
                    icon={<PlusOutlined />}
                    style={{ marginBottom: 12 }}
                    onClick={() => {
                      setEditingFaction(null)
                      factionForm.resetFields()
                      setFactionOpen(true)
                    }}
                  >
                    新增势力
                  </Button>
                  <Table<Faction>
                    rowKey="id"
                    loading={loading}
                    dataSource={factions}
                    columns={factionColumns}
                    pagination={false}
                  />
                </>
              ),
            },
          ]}
        />
      </Card>

      <Modal
        open={ruleOpen}
        title={editingRule ? `编辑规则：${editingRule.name}` : '新增世界规则'}
        okText="保存"
        cancelText="取消"
        onOk={submitRule}
        onCancel={() => setRuleOpen(false)}
        destroyOnHidden
      >
        <Form form={ruleForm} name="rule" layout="vertical" preserve={false}>
          <Form.Item name="name" label="规则名称" rules={[{ required: true, message: '请输入规则名称' }]}>
            <Input placeholder="例如：灵力不可凭空产生" maxLength={200} />
          </Form.Item>
          <Form.Item name="category" label="分类">
            <Input placeholder="力量体系 / 政治 / 宗教 / 地理…" maxLength={60} />
          </Form.Item>
          <Form.Item name="importance" label="重要度（1-5）">
            <Slider min={1} max={5} step={1} marks={{ 1: '1', 3: '3', 5: '5' }} />
          </Form.Item>
          <Form.Item name="description" label="说明">
            <Input.TextArea rows={3} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        open={locationOpen}
        title={editingLocation ? `编辑地点：${editingLocation.name}` : '新增地点'}
        okText="保存"
        cancelText="取消"
        onOk={submitLocation}
        onCancel={() => setLocationOpen(false)}
        destroyOnHidden
      >
        <Form form={locationForm} name="location" layout="vertical" preserve={false}>
          <Form.Item name="name" label="地点名称" rules={[{ required: true, message: '请输入地点名称' }]}>
            <Input placeholder="例如：青云城" maxLength={200} />
          </Form.Item>
          <Form.Item name="type" label="类型">
            <Input placeholder="城市 / 山脉 / 秘境…" maxLength={60} />
          </Form.Item>
          <Form.Item
            name="parent_location_id"
            label="上级地点"
            extra={editingLocation ? '已自动排除自己与自己的下级，避免形成环' : '留空表示顶层地点'}
          >
            <Select allowClear options={locationOptions} placeholder="（顶层）" showSearch optionFilterProp="label" />
          </Form.Item>
          <Form.Item name="description" label="说明">
            <Input.TextArea rows={3} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        open={factionOpen}
        title={editingFaction ? `编辑势力：${editingFaction.name}` : '新增势力'}
        okText="保存"
        cancelText="取消"
        onOk={submitFaction}
        onCancel={() => setFactionOpen(false)}
        destroyOnHidden
      >
        <Form form={factionForm} name="faction" layout="vertical" preserve={false}>
          <Form.Item name="name" label="势力名称" rules={[{ required: true, message: '请输入势力名称' }]}>
            <Input placeholder="例如：天枢阁" maxLength={200} />
          </Form.Item>
          <Form.Item name="type" label="类型">
            <Input placeholder="宗门 / 王朝 / 商会…" maxLength={60} />
          </Form.Item>
          <Form.Item name="goals" label="目标">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item name="relationships" label="与其他势力的关系">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item name="description" label="说明">
            <Input.TextArea rows={2} />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
