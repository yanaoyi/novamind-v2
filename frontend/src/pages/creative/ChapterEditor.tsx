import {
  HistoryOutlined,
  RobotOutlined,
  SaveOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons'
import {
  Alert,
  Button,
  Card,
  Col,
  Collapse,
  Drawer,
  Empty,
  Form,
  Input,
  InputNumber,
  List,
  Modal,
  Popconfirm,
  Progress,
  Row,
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

import { pollTask } from '../../api/taskPoll'
import { writingApi } from '../../api/writing'
import {
  CHAPTER_STATUS_COLOR,
  CHAPTER_STATUS_LABEL,
  REWRITE_ACTIONS,
  type ChapterStatus,
  type ChapterVersion,
  type CreativeChapter,
  type CreativeScene,
  type CreativeVolume,
  type RewriteAction,
} from '../../api/types'

const AUTOSAVE_DELAY = 1500

interface Draft {
  title: string
  summary: string
  content: string
  status: ChapterStatus
  volume_id: string | null
  purpose: string
  conflict: string
  outcome: string
}

function toDraft(c: CreativeChapter): Draft {
  return {
    title: c.title,
    summary: c.summary,
    content: c.content,
    status: c.status,
    volume_id: c.volume_id,
    purpose: c.purpose,
    conflict: c.conflict,
    outcome: c.outcome,
  }
}

interface Props {
  chapter: CreativeChapter
  volumes: CreativeVolume[]
  /** 章节被保存 / AI 重写 / 恢复版本后通知外层刷新列表 */
  onChanged: (chapter: CreativeChapter) => void
  onDeleted: () => void
}

export default function ChapterEditor({ chapter, volumes, onChanged, onDeleted }: Props) {
  const [draft, setDraft] = useState<Draft>(() => toDraft(chapter))
  const [saving, setSaving] = useState(false)
  const [savedAt, setSavedAt] = useState<string | null>(null)
  const [autoEnabled, setAutoEnabled] = useState(true)

  const [rewriteAction, setRewriteAction] = useState<RewriteAction>('润色')
  const [rewriteInstruction, setRewriteInstruction] = useState('')
  const [rewriting, setRewriting] = useState(false)

  const [generating, setGenerating] = useState(false)
  const [generateProgress, setGenerateProgress] = useState(0)
  const [generateMessage, setGenerateMessage] = useState('')
  const [generateOpen, setGenerateOpen] = useState(false)
  const [generateForm] = Form.useForm<{ target_words?: number; instruction?: string }>()

  const [versionsOpen, setVersionsOpen] = useState(false)
  const [versions, setVersions] = useState<ChapterVersion[]>([])
  const [versionsLoading, setVersionsLoading] = useState(false)
  const [preview, setPreview] = useState<ChapterVersion | null>(null)

  const [scenesOpen, setScenesOpen] = useState(false)
  const [scenes, setScenes] = useState<CreativeScene[]>([])
  const [sceneForm] = Form.useForm<{ title: string; location?: string; purpose?: string; conflict?: string; emotional_goal?: string; content?: string }>()

  // 用 ref 记录"用户是否改过"，避免刚加载就被自动保存覆盖
  const dirtyRef = useRef(false)
  const chapterIdRef = useRef(chapter.id)

  useEffect(() => {
    if (chapterIdRef.current === chapter.id) return
    chapterIdRef.current = chapter.id
    dirtyRef.current = false
    setDraft(toDraft(chapter))
    setSavedAt(null)
  }, [chapter])

  const patch = useCallback((part: Partial<Draft>) => {
    dirtyRef.current = true
    setDraft((prev) => ({ ...prev, ...part }))
  }, [])

  const save = useCallback(
    async (silent = false) => {
      const current = draft
      const body = {
        title: current.title,
        summary: current.summary,
        content: current.content,
        status: current.status,
        volume_id: current.volume_id,
        purpose: current.purpose,
        conflict: current.conflict,
        outcome: current.outcome,
      }
      setSaving(true)
      try {
        const res = await writingApi.updateChapter(chapter.id, body)
        dirtyRef.current = false
        setSavedAt(new Date().toLocaleTimeString('zh-CN'))
        onChanged(res.chapter)
        if (!silent) message.success(res.version_created ? '已保存（正文已存为新版本）' : '已保存')
        return res.chapter
      } catch (err) {
        message.error((err as Error).message)
        throw err
      } finally {
        setSaving(false)
      }
    },
    [chapter.id, draft, onChanged],
  )

  // 自动保存：停止输入 1.5 秒后落库
  useEffect(() => {
    if (!autoEnabled || !dirtyRef.current) return
    const timer = window.setTimeout(() => {
      void save(true).catch(() => undefined)
    }, AUTOSAVE_DELAY)
    return () => window.clearTimeout(timer)
  }, [draft, autoEnabled, save])

  const runRewrite = async () => {
    if (!draft.content.trim()) {
      message.warning('正文为空，先用「让 AI 写本章」起草')
      return
    }
    setRewriting(true)
    try {
      const res = await writingApi.rewrite(chapter.id, draft.content, rewriteAction, rewriteInstruction)
      patch({ content: res.text })
      message.success(`已按「${rewriteAction}」处理，保存后即为新版本`)
    } catch (err) {
      message.error((err as Error).message)
    } finally {
      setRewriting(false)
    }
  }

  const runGenerate = async () => {
    const values = await generateForm.validateFields()
    setGenerateOpen(false)
    setGenerating(true)
    setGenerateProgress(0)
    setGenerateMessage('已入队')
    try {
      const task = await writingApi.generateChapter(chapter.id, values)
      const finished = await pollTask(task.id, (t) => {
        setGenerateProgress(t.progress)
        setGenerateMessage(t.progress_message || t.status)
      })
      if (finished.status !== 'COMPLETED') {
        throw new Error(finished.error || '生成失败')
      }
      const fresh = await writingApi.getChapter(chapter.id)
      dirtyRef.current = false
      setDraft(toDraft(fresh))
      onChanged(fresh)
      message.success('AI 已写完本章并存入新版本')
    } catch (err) {
      message.error((err as Error).message)
    } finally {
      setGenerating(false)
    }
  }

  const openVersions = async () => {
    setVersionsOpen(true)
    setVersionsLoading(true)
    try {
      const list = await writingApi.listVersions(chapter.id)
      setVersions(list.items)
    } catch (err) {
      message.error((err as Error).message)
    } finally {
      setVersionsLoading(false)
    }
  }

  const openPreview = async (versionNo: number) => {
    try {
      setPreview(await writingApi.getVersion(chapter.id, versionNo))
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  const restore = async (versionNo: number) => {
    try {
      const restored = await writingApi.restoreVersion(chapter.id, versionNo)
      dirtyRef.current = false
      setDraft(toDraft(restored))
      onChanged(restored)
      message.success(`已恢复到 v${versionNo}（恢复前的内容也留了一版）`)
      const list = await writingApi.listVersions(chapter.id)
      setVersions(list.items)
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  const openScenes = async () => {
    setScenesOpen(true)
    try {
      const list = await writingApi.listScenes(chapter.id)
      setScenes(list.items)
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  const addScene = async () => {
    const values = await sceneForm.validateFields()
    try {
      await writingApi.createScene(chapter.id, values)
      sceneForm.resetFields()
      const list = await writingApi.listScenes(chapter.id)
      setScenes(list.items)
      message.success('场景已新增')
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  const versionColumns: ColumnsType<ChapterVersion> = [
    { title: '版本', dataIndex: 'version_no', width: 70, render: (no: number) => `v${no}` },
    { title: '字数', dataIndex: 'word_count', width: 80 },
    { title: '说明', dataIndex: 'note', ellipsis: true },
    {
      title: '时间',
      dataIndex: 'created_at',
      width: 160,
      render: (v: string) => new Date(v).toLocaleString('zh-CN'),
    },
    {
      title: '操作',
      width: 150,
      render: (_, row) => (
        <Space size="small">
          <Button size="small" onClick={() => void openPreview(row.version_no)}>
            预览
          </Button>
          <Popconfirm
            title={`恢复到 v${row.version_no}？`}
            description="当前正文会先自动存一版，不会丢内容。"
            onConfirm={() => void restore(row.version_no)}
          >
            <Button size="small" type="link">
              恢复
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <Card
      title={
        <Space>
          <Typography.Text strong>
            第 {chapter.chapter_no} 章 · {chapter.title}
          </Typography.Text>
          <Tag color={CHAPTER_STATUS_COLOR[chapter.status]}>{CHAPTER_STATUS_LABEL[chapter.status]}</Tag>
        </Space>
      }
      extra={
        <Space wrap>
          <Typography.Text type="secondary">
            {savedAt ? `已自动保存 ${savedAt}` : '未保存'}
          </Typography.Text>
          <Button icon={<SaveOutlined />} onClick={() => void save()} loading={saving}>
            保存
          </Button>
          <Button icon={<HistoryOutlined />} onClick={() => void openVersions()}>
            版本
          </Button>
          <Button icon={<ThunderboltOutlined />} onClick={() => void openScenes()}>
            场景
          </Button>
          <Button type="primary" icon={<RobotOutlined />} onClick={() => setGenerateOpen(true)} loading={generating}>
            让 AI 写本章
          </Button>
        </Space>
      }
    >
      {generating && (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="AI 正在写本章（可在任务中心查看）"
          description={
            <Progress percent={generateProgress} status="active" format={(p) => `${p}% · ${generateMessage}`} />
          }
        />
      )}

      <Row gutter={12} style={{ marginBottom: 12 }}>
        <Col span={10}>
          <Input
            addonBefore="标题"
            value={draft.title}
            onChange={(e) => patch({ title: e.target.value })}
          />
        </Col>
        <Col span={6}>
          <Select
            style={{ width: '100%' }}
            value={draft.volume_id ?? ''}
            onChange={(v) => patch({ volume_id: v === '' ? null : v })}
            options={[{ value: '', label: '（未归卷）' }, ...volumes.map((v) => ({ value: v.id, label: v.title }))]}
          />
        </Col>
        <Col span={4}>
          <Select
            style={{ width: '100%' }}
            value={draft.status}
            onChange={(v) => patch({ status: v as ChapterStatus })}
            options={(Object.keys(CHAPTER_STATUS_LABEL) as ChapterStatus[]).map((k) => ({
              value: k,
              label: CHAPTER_STATUS_LABEL[k],
            }))}
          />
        </Col>
        <Col span={4}>
          <Space>
            <Tooltip title="关闭后只手动保存">
              <Button size="small" type={autoEnabled ? 'primary' : 'default'} onClick={() => setAutoEnabled((v) => !v)}>
                自动保存{autoEnabled ? '开' : '关'}
              </Button>
            </Tooltip>
            <Popconfirm
              title="删除本章？"
              description="章节与它的版本会一起删除。"
              onConfirm={async () => {
                try {
                  await writingApi.removeChapter(chapter.id)
                  message.success('章节已删除')
                  onDeleted()
                } catch (err) {
                  message.error((err as Error).message)
                }
              }}
            >
              <Button size="small" danger>
                删除
              </Button>
            </Popconfirm>
          </Space>
        </Col>
      </Row>

      <Collapse
        size="small"
        style={{ marginBottom: 12 }}
        items={[
          {
            key: 'outline',
            label: '本章大纲（目的 / 冲突 / 结果）',
            children: (
              <Row gutter={12}>
                <Col span={8}>
                  <Input.TextArea
                    rows={2}
                    placeholder="目的：这一章要推进什么"
                    value={draft.purpose}
                    onChange={(e) => patch({ purpose: e.target.value })}
                  />
                </Col>
                <Col span={8}>
                  <Input.TextArea
                    rows={2}
                    placeholder="冲突：谁和谁、因为什么对立"
                    value={draft.conflict}
                    onChange={(e) => patch({ conflict: e.target.value })}
                  />
                </Col>
                <Col span={8}>
                  <Input.TextArea
                    rows={2}
                    placeholder="结果：冲突怎么收束"
                    value={draft.outcome}
                    onChange={(e) => patch({ outcome: e.target.value })}
                  />
                </Col>
                <Col span={24} style={{ marginTop: 8 }}>
                  <Input
                    addonBefore="摘要"
                    value={draft.summary}
                    onChange={(e) => patch({ summary: e.target.value })}
                  />
                </Col>
              </Row>
            ),
          },
        ]}
      />

      <Input.TextArea
        value={draft.content}
        onChange={(e) => patch({ content: e.target.value })}
        autoSize={{ minRows: 16, maxRows: 40 }}
        placeholder="在此写正文，或点右上角「让 AI 写本章」起草"
      />

      <Space wrap style={{ marginTop: 12 }}>
        <Tag>字数：{draft.content.replace(/\s/g, '').length}</Tag>
        <Select
          style={{ width: 140 }}
          value={rewriteAction}
          onChange={(v) => setRewriteAction(v as RewriteAction)}
          options={REWRITE_ACTIONS.map((a) => ({ value: a, label: `AI ${a}` }))}
        />
        <Input
          style={{ width: 260 }}
          placeholder="补充要求（可选）"
          value={rewriteInstruction}
          onChange={(e) => setRewriteInstruction(e.target.value)}
        />
        <Button onClick={() => void runRewrite()} loading={rewriting}>
          执行 AI 操作
        </Button>
      </Space>

      <Drawer title="版本历史" width={720} open={versionsOpen} onClose={() => setVersionsOpen(false)}>
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="每次保存正文都会自动留一版；恢复旧版本时，当前内容也会先存一版。"
        />
        <Table
          rowKey="id"
          size="small"
          loading={versionsLoading}
          columns={versionColumns}
          dataSource={versions}
          pagination={false}
          locale={{ emptyText: '还没有版本（写入正文后自动生成）' }}
        />
      </Drawer>

      <Modal
        title={preview ? `v${preview.version_no} 预览` : '版本预览'}
        open={preview !== null}
        onCancel={() => setPreview(null)}
        footer={null}
        width={760}
      >
        <pre style={{ whiteSpace: 'pre-wrap', maxHeight: 480, overflow: 'auto', margin: 0 }}>
          {preview?.content}
        </pre>
      </Modal>

      <Drawer title="场景（§29）" width={720} open={scenesOpen} onClose={() => setScenesOpen(false)}>
        <Form form={sceneForm} layout="vertical">
          <Row gutter={8}>
            <Col span={8}>
              <Form.Item name="title" label="场景标题" rules={[{ required: true, message: '请输入标题' }]}>
                <Input placeholder="如：月台初遇" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="location" label="地点">
                <Input placeholder="如：江城站" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="emotional_goal" label="情绪目标">
                <Input placeholder="如：克制的紧张" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="purpose" label="场景目的">
                <Input />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="conflict" label="场景冲突">
                <Input />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item name="content" label="场景正文">
                <Input.TextArea rows={3} />
              </Form.Item>
            </Col>
          </Row>
          <Button type="primary" onClick={() => void addScene()}>
            新增场景
          </Button>
        </Form>

        <List
          style={{ marginTop: 16 }}
          header={<Typography.Text strong>已登记 {scenes.length} 个场景</Typography.Text>}
          dataSource={scenes}
          locale={{ emptyText: <Empty description="还没有场景" /> }}
          renderItem={(s) => (
            <List.Item>
              <List.Item.Meta
                title={`${s.sequence}. ${s.title}`}
                description={
                  <Space wrap>
                    {s.location && <Tag>地点：{s.location}</Tag>}
                    {s.conflict && <Tag color="orange">冲突：{s.conflict}</Tag>}
                    {s.emotional_goal && <Tag color="blue">情绪：{s.emotional_goal}</Tag>}
                  </Space>
                }
              />
            </List.Item>
          )}
        />
      </Drawer>

      <Modal
        title="让 AI 写本章"
        open={generateOpen}
        onCancel={() => setGenerateOpen(false)}
        onOk={() => void runGenerate()}
        confirmLoading={generating}
        okText="开始生成"
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="会带上本章大纲、相关人物 DNA、世界规则、前几章摘要一起交给模型。"
        />
        <Form form={generateForm} layout="vertical" initialValues={{ target_words: 2000 }}>
          <Form.Item name="target_words" label="目标字数">
            <InputNumber min={200} max={10000} step={100} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="instruction" label="补充要求">
            <Input.TextArea rows={3} placeholder="如：保持克制的短句节奏，不要出现旁白解释" />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  )
}
