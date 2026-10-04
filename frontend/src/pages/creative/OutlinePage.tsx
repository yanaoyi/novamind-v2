import {
  DeleteOutlined,
  EditOutlined,
  FileAddOutlined,
  PlusOutlined,
  ReloadOutlined,
  RobotOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons'
import {
  Alert,
  Button,
  Card,
  Col,
  Collapse,
  Descriptions,
  Divider,
  Empty,
  Form,
  Input,
  List,
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Tag,
  Tree,
  Typography,
  message,
} from 'antd'
import type { DataNode } from 'antd/es/tree'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'

import { aiApi, outlineApi, type OutlineNodePatch } from '../../api/outline'
import type { Outline, OutlineDetail, OutlineNode, OutlineNodeInput } from '../../api/types'
import EntityVersions from '../../components/EntityVersions'
import { aiOutlineToNodeInputs, countOutlineInputs } from './outlineAi'
import { useCreativeWorks } from './useCreativeWorks'

const LEVEL_COLOR: Record<number, string> = { 1: 'geekblue', 2: 'purple', 3: 'green' }
const LEVEL_NAME = ['卷', '节', '章']

interface NodeModalState {
  open: boolean
  mode: 'create' | 'edit'
  parent: OutlineNode | null
  node: OutlineNode | null
}

/**
 * 二创 · 大纲（规格书 §27）：
 * 卷 → 节 → 章的结构先于章节存在；作者可以手搭、让 AI 出候选（改完再采纳），
 * 定稿后一键落成写作系统里的卷与章节。
 */
export default function OutlinePage() {
  const { originalWorkId, works, work, workId, setWorkId, loading: worksLoading, error } = useCreativeWorks()

  const [outlines, setOutlines] = useState<Outline[]>([])
  const [activeId, setActiveId] = useState<string | null>(null)
  const [detail, setDetail] = useState<OutlineDetail | null>(null)
  const [listLoading, setListLoading] = useState(false)
  const [busy, setBusy] = useState(false)
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null)

  const [createOpen, setCreateOpen] = useState(false)
  const [createForm] = Form.useForm<{ title: string; summary?: string; version?: number }>()

  const [nodeModal, setNodeModal] = useState<NodeModalState>({ open: false, mode: 'create', parent: null, node: null })
  const [nodeForm] = Form.useForm<{
    title: string
    summary?: string
    purpose?: string
    characters?: string
    location?: string
    conflict?: string
    outcome?: string
  }>()

  const [aiOpen, setAiOpen] = useState(false)
  const [aiInstruction, setAiInstruction] = useState('')
  const [aiCandidate, setAiCandidate] = useState<OutlineNodeInput[] | null>(null)
  const [aiRaw, setAiRaw] = useState('')
  const [aiLoading, setAiLoading] = useState(false)

  const loadOutlines = useCallback(
    async (targetId?: string) => {
      if (!workId) {
        setOutlines([])
        setActiveId(null)
        setDetail(null)
        return
      }
      setListLoading(true)
      try {
        const res = await outlineApi.list(workId)
        setOutlines(res.items)
        setActiveId((current) => {
          const wanted = targetId ?? current ?? res.items[0]?.id ?? null
          return res.items.some((o) => o.id === wanted) ? wanted : (res.items[0]?.id ?? null)
        })
      } catch (err) {
        message.error((err as Error).message)
      } finally {
        setListLoading(false)
      }
    },
    [workId],
  )

  const loadDetail = useCallback(async (id: string) => {
    try {
      setDetail(await outlineApi.get(id))
    } catch (err) {
      message.error((err as Error).message)
    }
  }, [])

  useEffect(() => {
    void loadOutlines()
  }, [loadOutlines])

  useEffect(() => {
    if (activeId) {
      void loadDetail(activeId)
    } else {
      setDetail(null)
    }
  }, [activeId, loadDetail])

  const refresh = useCallback(async () => {
    await loadOutlines(activeId ?? undefined)
    if (activeId) {
      await loadDetail(activeId)
    }
  }, [activeId, loadDetail, loadOutlines])

  const flatNodes = useMemo(() => {
    const out: OutlineNode[] = []
    const walk = (nodes: OutlineNode[]) => {
      for (const n of nodes) {
        out.push(n)
        walk(n.children ?? [])
      }
    }
    walk(detail?.nodes ?? [])
    return out
  }, [detail])

  const selectedNode = flatNodes.find((n) => n.id === selectedNodeId) ?? null

  const removeNode = useCallback(
    async (node: OutlineNode) => {
      setBusy(true)
      try {
        const res = await outlineApi.removeNode(node.id)
        message.success(`已删除 ${res.deleted_nodes} 个节点`)
        await refresh()
      } catch (err) {
        message.error((err as Error).message)
      } finally {
        setBusy(false)
      }
    },
    [refresh],
  )

  const openCreateNode = useCallback(
    (parent: OutlineNode | null) => {
      setNodeModal({ open: true, mode: 'create', parent, node: null })
      nodeForm.resetFields()
    },
    [nodeForm],
  )

  const openEditNode = useCallback(
    (node: OutlineNode) => {
      setNodeModal({ open: true, mode: 'edit', parent: null, node })
      nodeForm.setFieldsValue({
        title: node.title,
        summary: node.summary,
        purpose: node.purpose,
        characters: node.characters.join('、'),
        location: node.location,
        conflict: node.conflict,
        outcome: node.outcome,
      })
    },
    [nodeForm],
  )

  const treeData = useMemo(() => {
    const build = (nodes: OutlineNode[]): DataNode[] =>
      nodes.map((n) => ({
        key: n.id,
        title: (
          <Space size={6}>
            <Tag color={LEVEL_COLOR[n.level]} style={{ marginInlineEnd: 0 }}>
              {n.level_name}
            </Tag>
            <span>{n.title}</span>
            {n.level < 3 && (
              <Button type="link" size="small" icon={<PlusOutlined />} onClick={(e) => { e.stopPropagation(); openCreateNode(n) }}>
                子节点
              </Button>
            )}
            <Button type="link" size="small" icon={<EditOutlined />} onClick={(e) => { e.stopPropagation(); openEditNode(n) }}>
              编辑
            </Button>
            <Popconfirm title={`删除「${n.title}」及其全部子节点？`} onConfirm={() => void removeNode(n)} okText="删除" cancelText="取消">
              <Button type="link" size="small" danger icon={<DeleteOutlined />} onClick={(e) => e.stopPropagation()}>
                删除
              </Button>
            </Popconfirm>
          </Space>
        ),
        children: n.children?.length ? build(n.children) : undefined,
      }))
    return build(detail?.nodes ?? [])
  }, [detail, openCreateNode, openEditNode, removeNode])

  const submitCreate = async () => {
    const values = await createForm.validateFields()
    if (!workId) return
    setBusy(true)
    try {
      const created = await outlineApi.create(workId, {
        title: values.title,
        summary: values.summary,
        version: values.version,
        source: 'MANUAL',
      })
      message.success('大纲已创建：先加「卷」，再往下加「节」和「章」')
      setCreateOpen(false)
      createForm.resetFields()
      setActiveId(created.outline.id)
      await loadOutlines(created.outline.id)
    } catch (err) {
      message.error((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const removeOutline = async (outline: Outline) => {
    setBusy(true)
    try {
      await outlineApi.remove(outline.id)
      message.success('大纲已删除（删除前的结构会留一版快照）')
      setActiveId(null)
      await loadOutlines()
    } catch (err) {
      message.error((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const materialize = async () => {
    if (!activeId) return
    setBusy(true)
    try {
      const result = await outlineApi.materialize(activeId)
      if (result.chapters_created === 0) {
        message.warning('这份大纲里还没有「章」节点，先在「节」下面加几章')
      } else {
        message.success(
          `已落成：新建 ${result.volumes_created} 卷 / 复用 ${result.volumes_reused} 卷 / 追加 ${result.chapters_created} 章`,
        )
      }
    } catch (err) {
      message.error((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const submitNode = async () => {
    const values = await nodeForm.validateFields()
    const payload: OutlineNodePatch = {
      title: values.title,
      summary: values.summary ?? '',
      purpose: values.purpose ?? '',
      characters: (values.characters ?? '')
        .split(/[、,，\s]+/)
        .map((s) => s.trim())
        .filter(Boolean),
      location: values.location ?? '',
      conflict: values.conflict ?? '',
      outcome: values.outcome ?? '',
    }
    setBusy(true)
    try {
      if (nodeModal.mode === 'edit' && nodeModal.node) {
        await outlineApi.updateNode(nodeModal.node.id, payload)
        message.success('节点已更新')
      } else if (activeId) {
        await outlineApi.createNode(activeId, { ...payload, title: values.title, parent_id: nodeModal.parent?.id ?? null })
        message.success(nodeModal.parent ? `已在「${nodeModal.parent.title}」下新增子节点` : '已新增卷')
      }
      setNodeModal({ open: false, mode: 'create', parent: null, node: null })
      nodeForm.resetFields()
      await refresh()
    } catch (err) {
      message.error((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const generate = async () => {
    if (!workId) return
    setAiLoading(true)
    try {
      const payload = await aiApi.generate('outline', { workId, instruction: aiInstruction })
      const nodes = aiOutlineToNodeInputs(payload)
      setAiRaw(JSON.stringify(payload, null, 2))
      if (nodes.length === 0) {
        setAiCandidate(null)
        message.warning('模型没有返回可用的卷/节/章结构，改一下要求再试')
      } else {
        setAiCandidate(nodes)
        const { volumes, sections, chapters } = countOutlineInputs(nodes)
        message.success(`模型给出 ${volumes} 卷 / ${sections} 节 / ${chapters} 章，确认后再采纳`)
      }
    } catch (err) {
      message.error((err as Error).message)
    } finally {
      setAiLoading(false)
    }
  }

  const adoptCandidate = async () => {
    if (!workId || !aiCandidate) return
    setBusy(true)
    try {
      const created = await outlineApi.create(workId, {
        title: `AI 大纲 · ${new Date().toLocaleString('zh-CN', { hour12: false })}`,
        summary: aiInstruction,
        source: 'AI',
        nodes: aiCandidate,
      })
      message.success('已采纳为新大纲：之后随便改，AI 只出候选、不动你的正文')
      setAiOpen(false)
      setAiCandidate(null)
      setAiRaw('')
      setAiInstruction('')
      setActiveId(created.outline.id)
      await loadOutlines(created.outline.id)
    } catch (err) {
      message.error((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const candidateTree = useMemo(() => {
    if (!aiCandidate) return []
    const build = (nodes: OutlineNodeInput[], level: number): DataNode[] =>
      nodes.map((n, index) => ({
        key: `${level}-${index}-${n.title}`,
        title: (
          <Space size={6}>
            <Tag color={LEVEL_COLOR[level]} style={{ marginInlineEnd: 0 }}>
              {LEVEL_NAME[level - 1] ?? '章'}
            </Tag>
            <span>{n.title}</span>
          </Space>
        ),
        children: n.children?.length ? build(n.children, level + 1) : undefined,
      }))
    return build(aiCandidate, 1)
  }, [aiCandidate])

  if (!originalWorkId) {
    return (
      <Card>
        <Empty description="还没有选中的原著">
          <Space direction="vertical">
            <Typography.Text type="secondary">
              大纲属于二创作品，而二创作品从原著派生。先到「原著 · 总览」导入一本原文。
            </Typography.Text>
            <Link to="/original/overview">
              <Button type="primary">去原著 · 总览</Button>
            </Link>
          </Space>
        </Empty>
      </Card>
    )
  }

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Card>
        <Space wrap align="center">
          <Typography.Title level={4} style={{ margin: 0 }}>
            二创 · 大纲
          </Typography.Title>
          <Typography.Text type="secondary">
            卷 → 节 → 章：先定结构，再落成章节。AI 只出候选，采纳与否由你决定。
          </Typography.Text>
        </Space>
        <Divider style={{ margin: '12px 0' }} />
        <Space wrap>
          <Select
            style={{ minWidth: 260 }}
            loading={worksLoading}
            placeholder="选择二创作品"
            value={workId ?? undefined}
            onChange={(v) => setWorkId(v)}
            options={works.map((w) => ({ value: w.id, label: w.title }))}
          />
          <Button icon={<ReloadOutlined />} onClick={() => void refresh()} disabled={!workId}>
            刷新
          </Button>
          {error && <Typography.Text type="danger">{error}</Typography.Text>}
          {work && <Typography.Text type="secondary">当前作品：{work.title}</Typography.Text>}
        </Space>
      </Card>

      {!workId ? (
        <Card>
          <Empty description="当前原著下还没有二创作品">
            <Link to="/creative/overview">
              <Button type="primary">去二创 · 总览一键开同人</Button>
            </Link>
          </Empty>
        </Card>
      ) : (
        <Row gutter={16}>
          <Col span={8}>
            <Card
              title="大纲列表"
              loading={listLoading}
              extra={
                <Space size={4}>
                  <Button size="small" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>
                    新建
                  </Button>
                  <Button
                    size="small"
                    type="primary"
                    ghost
                    icon={<RobotOutlined />}
                    onClick={() => {
                      setAiCandidate(null)
                      setAiRaw('')
                      setAiOpen(true)
                    }}
                  >
                    AI 生成
                  </Button>
                </Space>
              }
            >
              {outlines.length === 0 ? (
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有大纲：手搭一份，或让 AI 先出候选" />
              ) : (
                <List
                  dataSource={outlines}
                  renderItem={(item) => (
                    <List.Item
                      style={{ cursor: 'pointer', background: item.id === activeId ? '#f0f5ff' : undefined, paddingInline: 8 }}
                      onClick={() => setActiveId(item.id)}
                      actions={[
                        <Popconfirm key="del" title="删除这份大纲？" onConfirm={() => void removeOutline(item)} okText="删除" cancelText="取消">
                          <Button type="text" size="small" danger icon={<DeleteOutlined />} />
                        </Popconfirm>,
                      ]}
                    >
                      <List.Item.Meta
                        title={
                          <Space size={6}>
                            <span>{item.title}</span>
                            <Tag color={item.source === 'AI' ? 'blue' : 'default'}>{item.source === 'AI' ? 'AI 候选' : '手写'}</Tag>
                          </Space>
                        }
                        description={
                          <Typography.Text type="secondary">
                            {item.node_count} 个节点 · v{item.version}
                            {item.summary ? ` · ${item.summary}` : ''}
                          </Typography.Text>
                        }
                      />
                    </List.Item>
                  )}
                />
              )}
            </Card>
          </Col>

          <Col span={16}>
            <Card
              title="大纲树"
              extra={
                <Space size={4}>
                  <Button size="small" icon={<PlusOutlined />} disabled={!activeId} onClick={() => openCreateNode(null)}>
                    新增卷
                  </Button>
                  <Button size="small" icon={<FileAddOutlined />} loading={busy} disabled={!activeId} onClick={() => void materialize()}>
                    落成章节
                  </Button>
                  {activeId && (
                    <EntityVersions entityType="creative_outline_tree" entityId={activeId} onRestored={refresh} label="版本" />
                  )}
                </Space>
              }
            >
              {!detail?.outline ? (
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="左侧选一份大纲" />
              ) : (
                <Space direction="vertical" size={12} style={{ width: '100%' }}>
                  <Descriptions size="small" column={2}>
                    <Descriptions.Item label="标题">{detail.outline.title}</Descriptions.Item>
                    <Descriptions.Item label="版本">v{detail.outline.version}</Descriptions.Item>
                    <Descriptions.Item label="概要" span={2}>
                      {detail.outline.summary || '—'}
                    </Descriptions.Item>
                  </Descriptions>

                  {detail.nodes.length === 0 ? (
                    <Alert
                      type="info"
                      showIcon
                      message="这份大纲还是空的"
                      description="点右上「新增卷」，再在卷下面加「节」、节下面加「章」——章是落成章节的单位。"
                    />
                  ) : (
                    <>
                      <Tree
                        treeData={treeData}
                        defaultExpandAll
                        selectedKeys={selectedNodeId ? [selectedNodeId] : []}
                        onSelect={(keys) => setSelectedNodeId((keys[0] as string) ?? null)}
                      />
                      <Button
                        size="small"
                        type="link"
                        icon={<ThunderboltOutlined />}
                        disabled={!selectedNode}
                        onClick={() => selectedNode && openCreateNode(selectedNode)}
                      >
                        在选中节点下加子节点
                      </Button>
                    </>
                  )}

                  {selectedNode && (
                    <Card size="small" title={`${selectedNode.level_name} · ${selectedNode.title}`}>
                      <Descriptions size="small" column={1}>
                        <Descriptions.Item label="概要">{selectedNode.summary || '—'}</Descriptions.Item>
                        <Descriptions.Item label="目的">{selectedNode.purpose || '—'}</Descriptions.Item>
                        <Descriptions.Item label="冲突">{selectedNode.conflict || '—'}</Descriptions.Item>
                        <Descriptions.Item label="结果 / 钩子">{selectedNode.outcome || '—'}</Descriptions.Item>
                        <Descriptions.Item label="地点">{selectedNode.location || '—'}</Descriptions.Item>
                        <Descriptions.Item label="人物">
                          {selectedNode.characters.length > 0 ? selectedNode.characters.map((c) => <Tag key={c}>{c}</Tag>) : '—'}
                        </Descriptions.Item>
                      </Descriptions>
                    </Card>
                  )}
                </Space>
              )}
            </Card>
          </Col>
        </Row>
      )}

      <Modal
        title="新建大纲"
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        onOk={() => void submitCreate()}
        confirmLoading={busy}
        okText="创建"
        destroyOnClose
      >
        <Form form={createForm} layout="vertical" initialValues={{ version: 1 }}>
          <Form.Item name="title" label="标题" rules={[{ required: true, message: '给大纲起个名字' }]}>
            <Input placeholder="例如：二创大纲 v1" />
          </Form.Item>
          <Form.Item name="summary" label="概要">
            <Input.TextArea rows={2} placeholder="这一版大纲的思路（可留空）" />
          </Form.Item>
          <Form.Item name="version" label="版本">
            <Input type="number" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={
          nodeModal.mode === 'edit'
            ? `编辑「${nodeModal.node?.title ?? ''}」`
            : nodeModal.parent
              ? `在「${nodeModal.parent.title}」下新增${nodeModal.parent.level === 1 ? '节' : '章'}`
              : '新增卷'
        }
        open={nodeModal.open}
        onCancel={() => setNodeModal({ open: false, mode: 'create', parent: null, node: null })}
        onOk={() => void submitNode()}
        confirmLoading={busy}
        okText="保存"
        destroyOnClose
      >
        <Form form={nodeForm} layout="vertical">
          <Form.Item name="title" label="标题" rules={[{ required: true, message: '节点标题不能为空' }]}>
            <Input placeholder="例如：第一卷 · 梅雨" />
          </Form.Item>
          <Form.Item name="summary" label="概要">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item name="purpose" label="叙事目的">
            <Input placeholder="这一章/节要达成什么" />
          </Form.Item>
          <Form.Item name="characters" label="出场人物" tooltip="用顿号或逗号分隔">
            <Input placeholder="林默、周叙" />
          </Form.Item>
          <Form.Item name="location" label="地点">
            <Input />
          </Form.Item>
          <Form.Item name="conflict" label="冲突">
            <Input />
          </Form.Item>
          <Form.Item name="outcome" label="结果 / 钩子">
            <Input />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="AI 生成大纲（候选）"
        width={760}
        open={aiOpen}
        onCancel={() => setAiOpen(false)}
        footer={[
          <Button key="cancel" onClick={() => setAiOpen(false)}>
            关闭
          </Button>,
          <Button key="gen" icon={<RobotOutlined />} loading={aiLoading} onClick={() => void generate()}>
            生成候选
          </Button>,
          <Button key="adopt" type="primary" disabled={!aiCandidate} loading={busy} onClick={() => void adoptCandidate()}>
            采纳为新大纲
          </Button>,
        ]}
      >
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <Alert
            type="info"
            showIcon
            message="AI 只产出候选，采纳之前不会写入任何数据"
            description="采纳后就是一份普通大纲：可以改标题、加删节点、回滚版本。"
          />
          <Input.TextArea
            rows={3}
            value={aiInstruction}
            onChange={(e) => setAiInstruction(e.target.value)}
            placeholder="创作要求，例如：三卷结构，主线是查清母亲意外，每卷结尾留一个反转"
          />
          {candidateTree.length > 0 && (
            <>
              <Typography.Text strong>候选结构</Typography.Text>
              <Tree treeData={candidateTree} defaultExpandAll selectable={false} />
            </>
          )}
          {aiRaw && (
            <Collapse
              size="small"
              items={[
                {
                  key: 'raw',
                  label: '模型原始输出（JSON）',
                  children: <Input.TextArea rows={10} readOnly value={aiRaw} style={{ fontFamily: 'monospace' }} />,
                },
              ]}
            />
          )}
        </Space>
      </Modal>
    </Space>
  )
}
