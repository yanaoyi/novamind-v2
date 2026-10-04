import { errorMessage } from '../../api/client'
import {
  CheckCircleOutlined,
  DownloadOutlined,
  FileAddOutlined,
  PlusOutlined,
  ReloadOutlined,
} from '@ant-design/icons'
import {
  Alert,
  Button,
  Card,
  Col,
  Dropdown,
  Empty,
  Form,
  Input,
  InputNumber,
  List,
  Modal,
  Row,
  Segmented,
  Select,
  Space,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'

import { pollTask } from '../../api/taskPoll'
import { downloadExport, writingApi } from '../../api/writing'
import EntityVersions from '../../components/EntityVersions'
import {
  CHAPTER_STATUS_COLOR,
  CHAPTER_STATUS_LABEL,
  type CreativeChapter,
  type CreativeVolume,
} from '../../api/types'
import ChapterEditor from './ChapterEditor'
import { useCreativeWorks } from './useCreativeWorks'

interface Props {
  /** 从左侧菜单进入时直接落到对应视图 */
  defaultTab?: 'outline' | 'chapters'
}

export default function WritingWorkspacePage({ defaultTab = 'chapters' }: Props) {
  const { originalWorkId, works, work, workId, setWorkId, loading: worksLoading, error, reload } = useCreativeWorks()

  const [tab, setTab] = useState<'outline' | 'chapters'>(defaultTab)
  const [volumes, setVolumes] = useState<CreativeVolume[]>([])
  const [chapters, setChapters] = useState<CreativeChapter[]>([])
  const [activeId, setActiveId] = useState<string | null>(null)
  const [detail, setDetail] = useState<CreativeChapter | null>(null)
  const [listLoading, setListLoading] = useState(false)
  const [checking, setChecking] = useState(false)
  const [openIssues, setOpenIssues] = useState<number | null>(null)

  const [volumeOpen, setVolumeOpen] = useState(false)
  const [volumeForm] = Form.useForm<{ title: string; summary?: string; sequence?: number }>()
  const [chapterOpen, setChapterOpen] = useState(false)
  const [chapterForm] = Form.useForm<{
    chapter_no: number
    title: string
    volume_id?: string
    summary?: string
    purpose?: string
    conflict?: string
    outcome?: string
    content?: string
  }>()

  const loadWorkData = useCallback(
    async (id: string, keepActive = true) => {
      setListLoading(true)
      try {
        const [vols, chs] = await Promise.all([writingApi.listVolumes(id), writingApi.listChapters(id)])
        setVolumes(vols.items)
        setChapters(chs.items)
        setActiveId((current) => (keepActive && current ? current : (chs.items[0]?.id ?? null)))
      } catch (err) {
        message.error(errorMessage(err))
      } finally {
        setListLoading(false)
      }
    },
    [],
  )

  useEffect(() => {
    if (!workId) {
      setVolumes([])
      setChapters([])
      setDetail(null)
      setActiveId(null)
      return
    }
    void loadWorkData(workId, false)
  }, [workId, loadWorkData])

  // 加载当前章节详情（含正文）
  useEffect(() => {
    if (!activeId) {
      setDetail(null)
      return
    }
    let cancelled = false
    void (async () => {
      try {
        const chapter = await writingApi.getChapter(activeId)
        if (!cancelled) setDetail(chapter)
      } catch (err) {
        if (!cancelled) message.error(errorMessage(err))
      }
    })()
    return () => {
      cancelled = true
    }
  }, [activeId])

  const sortedChapters = useMemo(
    () => [...chapters].sort((a, b) => a.chapter_no - b.chapter_no),
    [chapters],
  )

  const refreshIssues = useCallback(async () => {
    if (!workId) return
    try {
      const res = await writingApi.listIssues(workId, 'OPEN')
      setOpenIssues(res.total)
    } catch {
      setOpenIssues(null)
    }
  }, [workId])

  useEffect(() => {
    void refreshIssues()
  }, [refreshIssues])

  const createVolume = async () => {
    const values = await volumeForm.validateFields()
    if (!workId) return
    try {
      await writingApi.createVolume(workId, values)
      volumeForm.resetFields()
      setVolumeOpen(false)
      await loadWorkData(workId)
      message.success('卷已创建')
    } catch (err) {
      message.error(errorMessage(err))
    }
  }

  const createChapter = async () => {
    const values = await chapterForm.validateFields()
    if (!workId) return
    try {
      const created = await writingApi.createChapter(workId, {
        ...values,
        volume_id: values.volume_id ? values.volume_id : null,
      })
      chapterForm.resetFields()
      setChapterOpen(false)
      await loadWorkData(workId, false)
      setActiveId(created.id)
      setTab('chapters')
      message.success('章节已创建')
    } catch (err) {
      message.error(errorMessage(err))
    }
  }

  const runConsistencyCheck = async () => {
    if (!workId) return
    setChecking(true)
    try {
      const task = await writingApi.checkConsistency(workId)
      const finished = await pollTask(task.id)
      if (finished.status !== 'COMPLETED') throw new Error(finished.error || '一致性检查失败')
      await refreshIssues()
      message.success('一致性检查完成，结果见「一致性问题」')
    } catch (err) {
      message.error(errorMessage(err))
    } finally {
      setChecking(false)
    }
  }

  const handleExport = async (format: 'txt' | 'md' | 'docx') => {
    if (!workId) return
    try {
      await downloadExport(workId, format)
      message.success(`已导出 ${format.toUpperCase()}`)
    } catch (err) {
      message.error(errorMessage(err))
    }
  }

  if (!originalWorkId) {
    return (
      <Card title="写作工作台">
        <Alert
          type="warning"
          showIcon
          message="还没有选中原著"
          description={
            <span>
              写作是二创作品下的功能，请先到 <Link to="/original/overview">原著总览</Link> 选择或导入一部原著。
            </span>
          }
        />
      </Card>
    )
  }

  const toolbar = (
    <Space wrap>
      <Select
        style={{ minWidth: 240 }}
        loading={worksLoading}
        value={workId ?? undefined}
        placeholder="选择二创作品"
        onChange={(v) => setWorkId(v)}
        options={works.map((w) => ({ value: w.id, label: w.title }))}
      />
      <Button icon={<ReloadOutlined />} onClick={() => void reload(workId ?? undefined)}>
        刷新
      </Button>
      <Button icon={<PlusOutlined />} onClick={() => setVolumeOpen(true)} disabled={!workId}>
        新建卷
      </Button>
      <Button
        type="primary"
        icon={<FileAddOutlined />}
        onClick={() => {
          // 每次打开都重算章号（审查 P2：initialValues 只在首次挂载生效，
          // 取消后重开会显示上一次的旧章号，容易撞唯一约束）
          chapterForm.setFieldsValue({ chapter_no: (sortedChapters.at(-1)?.chapter_no ?? 0) + 1 })
          setChapterOpen(true)
        }}
        disabled={!workId}
      >
        新建章节
      </Button>
      <Button icon={<CheckCircleOutlined />} loading={checking} onClick={() => void runConsistencyCheck()} disabled={!workId}>
        一致性检查
      </Button>
      <Dropdown
        disabled={!workId}
        trigger={['click']}
        menu={{
          items: [
            { key: 'txt', label: '导出 TXT' },
            { key: 'md', label: '导出 Markdown' },
            { key: 'docx', label: '导出 Word (docx)' },
          ],
          onClick: ({ key }) => void handleExport(key as 'txt' | 'md' | 'docx'),
        }}
      >
        <Button icon={<DownloadOutlined />}>导出</Button>
      </Dropdown>
      <Tooltip title="待处理的一致性问题">
        <Tag color={openIssues ? 'red' : 'green'}>
          一致性问题：{openIssues === null ? '—' : openIssues}
        </Tag>
      </Tooltip>
      <Link to="/consistency">查看问题</Link>
    </Space>
  )

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Card title="写作工作台" extra={toolbar}>
        {error && <Alert type="error" showIcon message={error} style={{ marginBottom: 12 }} />}
        {!worksLoading && works.length === 0 && (
          <Alert
            type="info"
            showIcon
            message="这部原著还没有二创作品"
            description={
              <span>
                到 <Link to="/creative/characters">二创 · 人物</Link> 点右上角「新建二创作品」
                （从当前原著派生，会自动继承人物与世界观）后即可开始写。
              </span>
            }
          />
        )}
        {work && (
          <Space wrap>
            <Tag color="blue">当前作品：{work.title}</Tag>
            <Tag>{chapters.length} 章</Tag>
            <Tag>{volumes.length} 卷</Tag>
          </Space>
        )}
      </Card>

      {work && (
        <Card>
          <Segmented
            value={tab}
            onChange={(v) => setTab(v as 'outline' | 'chapters')}
            options={[
              { value: 'outline', label: '大纲与卷' },
              { value: 'chapters', label: '章节写作' },
            ]}
          />
          {tab === 'outline' && workId && (
            <Space style={{ marginLeft: 12 }}>
              <EntityVersions
                entityType="creative_outline"
                entityId={workId}
                label="大纲版本"
                onRestored={() => void loadWorkData(workId, true)}
              />
            </Space>
          )}

          {tab === 'outline' && (
            <Row gutter={16} style={{ marginTop: 16 }}>
              <Col span={8}>
                <Card size="small" title="卷">
                  <List
                    size="small"
                    dataSource={volumes}
                    locale={{ emptyText: '还没有卷' }}
                    renderItem={(v) => (
                      <List.Item>
                        <List.Item.Meta
                          title={`${v.sequence}. ${v.title}`}
                          description={v.summary || '—'}
                        />
                      </List.Item>
                    )}
                  />
                </Card>
              </Col>
              <Col span={16}>
                <Card size="small" title="章节大纲">
                  <List
                    size="small"
                    loading={listLoading}
                    dataSource={sortedChapters}
                    locale={{ emptyText: <Empty description="还没有章节，点上方「新建章节」" /> }}
                    renderItem={(c) => (
                      <List.Item
                        actions={[
                          <Button
                            key="edit"
                            type="link"
                            onClick={() => {
                              setActiveId(c.id)
                              setTab('chapters')
                            }}
                          >
                            去写
                          </Button>,
                        ]}
                      >
                        <List.Item.Meta
                          title={
                            <Space>
                              <span>
                                第 {c.chapter_no} 章 · {c.title}
                              </span>
                              <Tag color={CHAPTER_STATUS_COLOR[c.status]}>{CHAPTER_STATUS_LABEL[c.status]}</Tag>
                              <Tag>{c.word_count} 字</Tag>
                            </Space>
                          }
                          description={
                            <Space direction="vertical" size={0}>
                              <Typography.Text type="secondary">目的：{c.purpose || '—'}</Typography.Text>
                              <Typography.Text type="secondary">冲突：{c.conflict || '—'}</Typography.Text>
                              <Typography.Text type="secondary">结果：{c.outcome || '—'}</Typography.Text>
                            </Space>
                          }
                        />
                      </List.Item>
                    )}
                  />
                </Card>
              </Col>
            </Row>
          )}

          {tab === 'chapters' && (
            <Row gutter={16} style={{ marginTop: 16 }}>
              <Col span={6}>
                <Card size="small" title={`章节（${sortedChapters.length}）`}>
                  <List
                    size="small"
                    loading={listLoading}
                    dataSource={sortedChapters}
                    locale={{ emptyText: '还没有章节' }}
                    renderItem={(c) => (
                      <List.Item
                        onClick={() => setActiveId(c.id)}
                        style={{
                          cursor: 'pointer',
                          background: c.id === activeId ? 'rgba(22,119,255,0.08)' : undefined,
                          paddingInline: 8,
                          borderRadius: 4,
                        }}
                      >
                        <List.Item.Meta
                          title={
                            <Space size={4}>
                              <span>
                                {c.chapter_no}. {c.title}
                              </span>
                            </Space>
                          }
                          description={
                            <Space size={4}>
                              <Tag color={CHAPTER_STATUS_COLOR[c.status]}>{CHAPTER_STATUS_LABEL[c.status]}</Tag>
                              <Typography.Text type="secondary">{c.word_count} 字</Typography.Text>
                            </Space>
                          }
                        />
                      </List.Item>
                    )}
                  />
                </Card>
              </Col>
              <Col span={18}>
                {detail ? (
                  <ChapterEditor
                    chapter={detail}
                    volumes={volumes}
                    onChanged={(updated) => {
                      setDetail(updated)
                      setChapters((prev) => prev.map((c) => (c.id === updated.id ? { ...c, ...updated } : c)))
                    }}
                    onDeleted={() => {
                      setDetail(null)
                      setActiveId(null)
                      void loadWorkData(workId as string, false)
                    }}
                  />
                ) : (
                  <Card>
                    <Empty description="左侧选一章开始写" />
                  </Card>
                )}
              </Col>
            </Row>
          )}
        </Card>
      )}

      <Modal title="新建卷" open={volumeOpen} onCancel={() => setVolumeOpen(false)} onOk={() => void createVolume()}>
        <Form form={volumeForm} layout="vertical" initialValues={{ sequence: 1 }}>
          <Form.Item name="title" label="卷标题" rules={[{ required: true, message: '请输入卷标题' }]}>
            <Input placeholder="如：第一卷 · 梅雨" />
          </Form.Item>
          <Form.Item name="sequence" label="序号">
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="summary" label="卷简介">
            <Input.TextArea rows={2} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="新建章节"
        open={chapterOpen}
        onCancel={() => setChapterOpen(false)}
        onOk={() => void createChapter()}
        width={640}
      >
        <Form form={chapterForm} layout="vertical">
          <Row gutter={12}>
            <Col span={8}>
              <Form.Item name="chapter_no" label="章号" rules={[{ required: true, message: '请输入章号' }]}>
                <InputNumber min={1} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={16}>
              <Form.Item name="volume_id" label="所属卷">
                <Select
                  allowClear
                  placeholder="可不归卷"
                  options={volumes.map((v) => ({ value: v.id, label: v.title }))}
                />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="title" label="标题" rules={[{ required: true, message: '请输入标题' }]}>
            <Input placeholder="如：旧信" />
          </Form.Item>
          <Form.Item name="summary" label="摘要">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Row gutter={12}>
            <Col span={8}>
              <Form.Item name="purpose" label="目的">
                <Input.TextArea rows={2} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="conflict" label="冲突">
                <Input.TextArea rows={2} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="outcome" label="结果">
                <Input.TextArea rows={2} />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="content" label="正文（可留空，稍后用 AI 起草）">
            <Input.TextArea rows={4} />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
