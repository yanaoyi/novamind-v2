import { errorMessage } from '../../api/client'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import {
  Button,
  Card,
  Divider,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Slider,
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
import {
  DNA_KEYS,
  DNA_LABELS,
  RELATION_LABELS,
  emptyDNA,
  type Character,
  type CharacterDNA,
  type CharacterInput,
  type DNAKey,
  type RelationType,
  type Relationship,
} from '../../api/types'
import { useOriginalStore } from '../../stores/originalStore'

const IMPORTANCE_LABEL: Record<number, string> = {
  1: '路人',
  2: '次要',
  3: '重要',
  4: '主要',
  5: '核心',
}

const RELATION_TYPES = Object.keys(RELATION_LABELS) as RelationType[]

interface CharacterFormValues extends Omit<CharacterInput, 'dna'> {
  dna?: CharacterDNA
}

/** 把后端人物对象映射成表单值（补齐缺失的 DNA 维度，避免表单出现 undefined） */
function toFormValues(c: Character): CharacterFormValues {
  return {
    name: c.name,
    aliases: c.aliases ?? [],
    role: c.role,
    gender: c.gender,
    age: c.age,
    appearance: c.appearance,
    personality: c.personality,
    motivation: c.motivation,
    values: c.values,
    fears: c.fears,
    desires: c.desires,
    behavior_patterns: c.behavior_patterns,
    speech_style: c.speech_style,
    abilities: c.abilities,
    first_appearance: c.first_appearance,
    last_appearance: c.last_appearance,
    dna: { ...emptyDNA(), ...c.dna },
    importance: c.importance,
    notes: c.notes,
  }
}

interface RelationFormValues {
  source_character_id: string
  target_character_id: string
  relation_type: RelationType
  strength: number
  description?: string
}

/** DNA 摘要：显示前 3 个已设置权重的维度 */
function dnaSummary(dna?: CharacterDNA): string {
  if (!dna) return '—'
  const set = DNA_KEYS.map((key) => ({ key, ...dna[key] }))
    .filter((d) => d && d.weight > 0)
    .sort((a, b) => b.weight - a.weight)
    .slice(0, 3)
  if (set.length === 0) return '未设置'
  return set.map((d) => `${DNA_LABELS[d.key]} ${d.weight}%`).join(' · ')
}

export default function OriginalCharactersPage() {
  const { workId, work, loadWork } = useOriginalStore()
  const [characters, setCharacters] = useState<Character[]>([])
  const [relations, setRelations] = useState<Relationship[]>([])
  const [loading, setLoading] = useState(false)
  const [keyword, setKeyword] = useState('')

  const [charOpen, setCharOpen] = useState(false)
  const [editing, setEditing] = useState<Character | null>(null)
  const [charForm] = Form.useForm<CharacterFormValues>()

  const [relOpen, setRelOpen] = useState(false)
  const [relForm] = Form.useForm<RelationFormValues>()

  const reload = useCallback(async () => {
    if (!workId) return
    setLoading(true)
    try {
      const [list, rels] = await Promise.all([
        characterApi.list(workId, { pageSize: 200, keyword }),
        characterApi.listRelationships(workId),
      ])
      setCharacters(list.items)
      setRelations(rels.items)
    } catch (err) {
      message.error(errorMessage(err))
    } finally {
      setLoading(false)
    }
  }, [workId, keyword])

  useEffect(() => {
    if (!workId) return
    if (!work) void loadWork(workId).catch((err: Error) => message.error(err.message))
  }, [workId, work, loadWork])

  useEffect(() => {
    void reload()
  }, [reload])

  const openCreate = () => {
    setEditing(null)
    charForm.resetFields()
    charForm.setFieldsValue({
      importance: 3,
      dna: emptyDNA(),
      aliases: [],
    })
    setCharOpen(true)
  }

  const openEdit = (record: Character) => {
    setEditing(record)
    charForm.setFieldsValue(toFormValues(record))
    setCharOpen(true)
  }

  const submitCharacter = async () => {
    const values = await charForm.validateFields()
    if (!workId) return
    const input: CharacterInput = { ...values, dna: { ...emptyDNA(), ...(values.dna ?? {}) } }
    try {
      if (editing) {
        await characterApi.update(editing.id, input)
        message.success('人物已更新')
      } else {
        await characterApi.create(workId, input)
        message.success('人物已创建')
      }
      setCharOpen(false)
      await reload()
    } catch (err) {
      message.error(errorMessage(err))
    }
  }

  const removeCharacter = async (record: Character) => {
    try {
      await characterApi.remove(record.id)
      message.success(`已删除「${record.name}」`)
      await reload()
    } catch (err) {
      message.error(errorMessage(err))
    }
  }

  const openRelation = () => {
    relForm.resetFields()
    relForm.setFieldsValue({ relation_type: 'friend', strength: 50 })
    setRelOpen(true)
  }

  const submitRelation = async () => {
    const values = await relForm.validateFields()
    if (!workId) return
    try {
      await characterApi.createRelationship(workId, values)
      message.success('关系已建立')
      setRelOpen(false)
      await reload()
    } catch (err) {
      message.error(errorMessage(err))
    }
  }

  const removeRelation = async (record: Relationship) => {
    try {
      await characterApi.removeRelationship(record.id)
      await reload()
    } catch (err) {
      message.error(errorMessage(err))
    }
  }

  if (!workId) {
    return (
      <Card title="人物">
        <Empty
          description={<span>还没有选择原著，<Link to="/original/overview">去选择</Link></span>}
        />
      </Card>
    )
  }

  const nameOf = (id: string) => characters.find((c) => c.id === id)?.name ?? id.slice(0, 8)

  const characterColumns: ColumnsType<Character> = [
    {
      title: '姓名',
      dataIndex: 'name',
      key: 'name',
      width: 180,
      render: (value: string, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{value}</Typography.Text>
          {record.aliases?.length > 0 && (
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              别名：{record.aliases.join('、')}
            </Typography.Text>
          )}
        </Space>
      ),
    },
    { title: '角色', dataIndex: 'role', key: 'role', width: 100 },
    {
      title: '重要度',
      dataIndex: 'importance',
      key: 'importance',
      width: 100,
      render: (v: number) => <Tag color={v >= 4 ? 'red' : v === 3 ? 'blue' : 'default'}>{IMPORTANCE_LABEL[v] ?? v}</Tag>,
    },
    {
      title: '人物 DNA',
      key: 'dna',
      render: (_, record) => <Typography.Text type="secondary">{dnaSummary(record.dna)}</Typography.Text>,
    },
    {
      title: '操作',
      key: 'actions',
      width: 140,
      render: (_, record) => (
        <Space>
          <Button type="link" size="small" onClick={() => openEdit(record)}>
            编辑
          </Button>
          <Popconfirm title={`删除「${record.name}」？`} okText="确认删除" cancelText="取消" onConfirm={() => removeCharacter(record)}>
            <Button type="link" size="small" danger>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  const relationColumns: ColumnsType<Relationship> = [
    { title: '从', key: 'from', render: (_, r) => nameOf(r.source_character_id) },
    {
      title: '关系',
      dataIndex: 'relation_type',
      key: 'type',
      width: 110,
      render: (v: RelationType) => <Tag>{RELATION_LABELS[v] ?? v}</Tag>,
    },
    { title: '到', key: 'to', render: (_, r) => nameOf(r.target_character_id) },
    {
      title: '强度',
      dataIndex: 'strength',
      key: 'strength',
      width: 90,
      render: (v: number) => `${v}`,
    },
    { title: '说明', dataIndex: 'description', key: 'description', ellipsis: true },
    {
      title: '操作',
      key: 'actions',
      width: 80,
      render: (_, record) => (
        <Popconfirm title="删除这条关系？" okText="确认删除" cancelText="取消" onConfirm={() => removeRelation(record)}>
          <Button type="link" size="small" danger>
            删除
          </Button>
        </Popconfirm>
      ),
    },
  ]

  const characterOptions = characters.map((c) => ({ value: c.id, label: `${c.name}${c.role ? `（${c.role}）` : ''}` }))

  return (
    <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
      <Card
        title={work ? `《${work.title}》人物（${characters.length}）` : '人物'}
        extra={
          <Space>
            <Input.Search allowClear placeholder="按姓名/角色搜索" style={{ width: 220 }} onSearch={setKeyword} />
            <Button icon={<ReloadOutlined />} onClick={() => void reload()}>
              刷新
            </Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              新增人物
            </Button>
          </Space>
        }
      >
        {characters.length === 0 && !loading ? (
          <Empty description="还没有人物，点右上角「新增人物」开始" />
        ) : (
          <Table<Character> rowKey="id" loading={loading} dataSource={characters} columns={characterColumns} pagination={{ pageSize: 20 }} />
        )}
      </Card>

      <Card
        title={`人物关系（${relations.length}）`}
        extra={
          <Button type="primary" icon={<PlusOutlined />} onClick={openRelation} disabled={characters.length < 2}>
            建立关系
          </Button>
        }
      >
        {relations.length === 0 ? (
          <Empty description={characters.length < 2 ? '至少要有两个人物才能建立关系' : '还没有关系'} />
        ) : (
          <Table<Relationship> rowKey="id" dataSource={relations} columns={relationColumns} pagination={false} />
        )}
      </Card>

      <Modal
        open={charOpen}
        title={editing ? `编辑人物：${editing.name}` : '新增人物'}
        width={860}
        okText="保存"
        cancelText="取消"
        onOk={submitCharacter}
        onCancel={() => setCharOpen(false)}
        destroyOnHidden
      >
        <Form form={charForm} name="character" layout="vertical" preserve={false}>
          <Space size="middle" style={{ display: 'flex', alignItems: 'flex-start' }}>
            <Form.Item name="name" label="姓名" rules={[{ required: true, message: '请输入姓名' }]} style={{ minWidth: 200 }}>
              <Input placeholder="例如：林默" maxLength={120} />
            </Form.Item>
            <Form.Item name="aliases" label="别名" style={{ minWidth: 240 }}>
              <Select mode="tags" placeholder="输入后回车添加" open={false} suffixIcon={null} />
            </Form.Item>
            <Form.Item name="role" label="角色" style={{ minWidth: 140 }}>
              <Input placeholder="主角 / 配角" maxLength={40} />
            </Form.Item>
            <Form.Item name="importance" label="重要度" style={{ minWidth: 140 }}>
              <Select
                options={[5, 4, 3, 2, 1].map((v) => ({ value: v, label: `${v}｜${IMPORTANCE_LABEL[v]}` }))}
              />
            </Form.Item>
          </Space>

          <Space size="middle" style={{ display: 'flex' }}>
            <Form.Item name="gender" label="性别" style={{ minWidth: 120 }}>
              <Input maxLength={20} />
            </Form.Item>
            <Form.Item name="age" label="年龄" style={{ minWidth: 120 }}>
              <Input maxLength={40} />
            </Form.Item>
            <Form.Item name="first_appearance" label="首次出场" style={{ minWidth: 200 }}>
              <Input maxLength={200} />
            </Form.Item>
            <Form.Item name="last_appearance" label="最后出场" style={{ minWidth: 200 }}>
              <Input maxLength={200} />
            </Form.Item>
          </Space>

          <Space size="middle" style={{ display: 'flex', alignItems: 'flex-start' }}>
            <Form.Item name="appearance" label="外貌" style={{ minWidth: 260 }}>
              <Input.TextArea rows={2} />
            </Form.Item>
            <Form.Item name="personality" label="性格" style={{ minWidth: 260 }}>
              <Input.TextArea rows={2} />
            </Form.Item>
            <Form.Item name="motivation" label="动机" style={{ minWidth: 260 }}>
              <Input.TextArea rows={2} />
            </Form.Item>
          </Space>
          <Space size="middle" style={{ display: 'flex', alignItems: 'flex-start' }}>
            <Form.Item name="values" label="价值观" style={{ minWidth: 260 }}>
              <Input.TextArea rows={2} />
            </Form.Item>
            <Form.Item name="fears" label="恐惧" style={{ minWidth: 260 }}>
              <Input.TextArea rows={2} />
            </Form.Item>
            <Form.Item name="desires" label="欲望" style={{ minWidth: 260 }}>
              <Input.TextArea rows={2} />
            </Form.Item>
          </Space>
          <Space size="middle" style={{ display: 'flex', alignItems: 'flex-start' }}>
            <Form.Item name="behavior_patterns" label="行为模式" style={{ minWidth: 260 }}>
              <Input.TextArea rows={2} />
            </Form.Item>
            <Form.Item name="speech_style" label="语言风格" style={{ minWidth: 260 }}>
              <Input.TextArea rows={2} />
            </Form.Item>
            <Form.Item name="abilities" label="能力" style={{ minWidth: 260 }}>
              <Input.TextArea rows={2} />
            </Form.Item>
          </Space>
          <Form.Item name="notes" label="备注">
            <Input.TextArea rows={2} />
          </Form.Item>

          <Divider orientation="left" plain>
            人物 DNA（0-100 继承权重）
          </Divider>
          <Typography.Paragraph type="secondary" style={{ marginTop: -8 }}>
            权重表示"这个维度有多重要"。二创继承人物时按此比例保留特征——比如人格 90%、语言风格 30%。
          </Typography.Paragraph>
          {DNA_KEYS.map((key: DNAKey) => (
            <Space key={key} align="baseline" style={{ display: 'flex' }}>
              <Typography.Text style={{ width: 84, display: 'inline-block' }}>{DNA_LABELS[key]}</Typography.Text>
              <Form.Item name={['dna', key, 'text']} style={{ marginBottom: 8, width: 380 }}>
                <Input placeholder="一句描述（可空）" />
              </Form.Item>
              <Form.Item name={['dna', key, 'weight']} style={{ marginBottom: 8, width: 240 }}>
                <Slider min={0} max={100} step={5} />
              </Form.Item>
              <Form.Item name={['dna', key, 'weight']} style={{ marginBottom: 8, width: 80 }}>
                <InputNumber min={0} max={100} />
              </Form.Item>
            </Space>
          ))}
        </Form>
      </Modal>

      <Modal
        open={relOpen}
        title="建立人物关系"
        okText="建立"
        cancelText="取消"
        onOk={submitRelation}
        onCancel={() => setRelOpen(false)}
        destroyOnHidden
      >
        <Form form={relForm} name="relation" layout="vertical" preserve={false}>
          <Form.Item name="source_character_id" label="从" rules={[{ required: true, message: '请选择人物' }]}>
            <Select options={characterOptions} showSearch optionFilterProp="label" />
          </Form.Item>
          <Form.Item name="relation_type" label="关系" rules={[{ required: true }]}>
            <Select options={RELATION_TYPES.map((t) => ({ value: t, label: RELATION_LABELS[t] }))} />
          </Form.Item>
          <Form.Item name="target_character_id" label="到" rules={[{ required: true, message: '请选择人物' }]}>
            <Select options={characterOptions} showSearch optionFilterProp="label" />
          </Form.Item>
          <Form.Item name="strength" label="强度（0-100）">
            <Slider min={0} max={100} step={5} />
          </Form.Item>
          <Form.Item name="description" label="说明">
            <Input.TextArea rows={2} placeholder="例如：自幼相识，彼此信任" />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
