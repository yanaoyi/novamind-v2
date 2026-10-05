import { errorMessage } from '../../api/client'
import { InboxOutlined } from '@ant-design/icons'
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Form,
  Input,
  Select,
  Space,
  Tag,
  Typography,
  Upload,
  message,
} from 'antd'
import type { UploadFile } from 'antd/es/upload/interface'
import { useEffect, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'

import { projectApi } from '../../api/projects'
import type { Project } from '../../api/types'
import { useOriginalStore } from '../../stores/originalStore'

/** 与后端 UPLOAD_MAX_MB 默认值保持一致（50MB），超了在浏览器里就拦下 */
const MAX_UPLOAD_BYTES = 50 * 1024 * 1024

const STATUS_LABEL: Record<string, string> = {
  DRAFT: '待导入',
  PARSED: '已导入',
  ANALYZED: '已分析',
}

const STATUS_COLOR: Record<string, string> = {
  DRAFT: 'default',
  PARSED: 'green',
  ANALYZED: 'blue',
}

export default function OriginalOverviewPage() {
  const {
    workId,
    work,
    error,
    loadWork,
    selectByProject,
    createForProject,
    importFile,
    reset,
  } = useOriginalStore()

  const [projects, setProjects] = useState<Project[]>([])
  const [selectedProject, setSelectedProject] = useState<string>()
  const [needsCreate, setNeedsCreate] = useState(false)
  const [file, setFile] = useState<File | null>(null)
  const [fileList, setFileList] = useState<UploadFile[]>([])
  const [importing, setImporting] = useState(false)
  const [createForm] = Form.useForm<{ title: string; author?: string }>()
  const location = useLocation()
  const presetProjectId = (location.state as { projectId?: string } | null)?.projectId

  // 未选中原著时，列出所有 ORIGINAL 工程供选择
  useEffect(() => {
    if (workId) return
    projectApi
      .list({ type: 'ORIGINAL', page_size: 100 })
      .then((data) => setProjects(data.items))
      .catch((err: Error) => message.error(`加载工程失败：${err.message}`))
  }, [workId])

  // 有 workId 但还没加载详情时补一次
  useEffect(() => {
    if (!workId || (work && work.id === workId)) return
    void loadWork(workId).catch((err: Error) => message.error(err.message))
  }, [workId, work, loadWork])

  const onSelectProject = async (projectId: string) => {
    setSelectedProject(projectId)
    setNeedsCreate(false)
    try {
      const found = await selectByProject(projectId)
      if (!found) {
        setNeedsCreate(true)
        const project = projects.find((p) => p.id === projectId)
        createForm.setFieldsValue({ title: project?.name ?? '' })
      } else {
        message.success(`已切换到《${found.title}》`)
      }
    } catch (err) {
      message.error(errorMessage(err))
    }
  }

  // 从工程列表点「原著」进来时，自动定位到该工程
  useEffect(() => {
    if (workId || !presetProjectId || projects.length === 0) return
    if (selectedProject === presetProjectId) return
    void onSelectProject(presetProjectId)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [presetProjectId, projects, workId])

  const onCreate = async () => {
    if (!selectedProject) return
    try {
      const values = await createForm.validateFields()
      const created = await createForProject(selectedProject, values)
      setNeedsCreate(false)
      message.success(`原著《${created.title}》已创建，请上传原文`)
    } catch (err) {
      message.error(errorMessage(err))
    }
  }

  const onImport = async () => {
    if (!file) {
      message.warning('请先选择原著文件（.txt 或 .docx）')
      return
    }
    setImporting(true)
    try {
      const result = await importFile(file)
      message.success(
        `导入成功：识别编码 ${result.encoding}，切出 ${result.chapter_count} 章、${result.char_count.toLocaleString('zh-CN')} 字`,
      )
      setFile(null)
      setFileList([])
    } catch (err) {
      message.error(`导入失败：${errorMessage(err)}`)
    } finally {
      setImporting(false)
    }
  }

  if (error) {
    return <Alert type="error" showIcon message={error} />
  }

  // ---------- 尚未选中原著：先选工程，必要时创建 ----------
  if (!workId) {
    return (
      <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
        <Card title="选择原著工程">
          <Space direction="vertical" size="small" style={{ display: 'flex' }}>
            <Typography.Text type="secondary">
              原著绑定在「原著类型」的工程上。先选一个工程，再导入原文。
            </Typography.Text>
            <Select
              style={{ width: 420 }}
              placeholder={projects.length ? '请选择原著工程' : '还没有原著工程，请先到「文章管理」创建'}
              value={selectedProject}
              onChange={onSelectProject}
              options={projects.map((p) => ({ value: p.id, label: p.name }))}
              notFoundContent={<Link to="/projects">去创建工程</Link>}
            />
          </Space>
        </Card>

        {needsCreate && (
          <Card title="该工程还没有原著，创建一个">
            <Form form={createForm} layout="vertical" style={{ maxWidth: 520 }}>
              <Form.Item
                name="title"
                label="原著标题"
                rules={[{ required: true, message: '请输入原著标题' }]}
              >
                <Input placeholder="例如：暗涌" maxLength={200} />
              </Form.Item>
              <Form.Item name="author" label="作者">
                <Input placeholder="选填" maxLength={120} />
              </Form.Item>
              <Button type="primary" onClick={onCreate}>
                创建原著
              </Button>
            </Form>
          </Card>
        )}
      </Space>
    )
  }

  // ---------- 已选中原著 ----------
  if (!work) {
    return <Card loading title="加载原著…" />
  }

  return (
    <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
      <Card
        title={work.title}
        extra={
          <Space>
            <Button
              onClick={() => {
                reset()
                setSelectedProject(undefined)
                setNeedsCreate(false)
              }}
            >
              切换原著
            </Button>
            <Link to="/original/chapters">
              <Button type="primary" disabled={work.chapter_count === 0}>
                查看章节（{work.chapter_count}）
              </Button>
            </Link>
          </Space>
        }
      >
        <Descriptions column={2} size="small">
          <Descriptions.Item label="作者">{work.author || '—'}</Descriptions.Item>
          <Descriptions.Item label="状态">
            <Tag color={STATUS_COLOR[work.status]}>{STATUS_LABEL[work.status] ?? work.status}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label="来源">{work.source_type}</Descriptions.Item>
          <Descriptions.Item label="字数">{work.char_count.toLocaleString('zh-CN')}</Descriptions.Item>
          <Descriptions.Item label="章节数">{work.chapter_count}</Descriptions.Item>
          <Descriptions.Item label="更新时间">
            {new Date(work.updated_at).toLocaleString('zh-CN')}
          </Descriptions.Item>
        </Descriptions>
      </Card>

      <Card title="导入原文" size="small">
        <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
          <Alert
            type="info"
            showIcon
            message="支持 .txt 与 .docx"
            description={
              <span>
                文本编码自动识别（UTF-8 / GB18030 / Big5 等）；章节按「第X章」「Chapter N」「序章/楔子/尾声」等标题自动切分，
                识别不到标题会按长度兜底切分。<b>重复导入会整体替换章节，不会重复累积。</b>
              </span>
            }
          />
          <Upload.Dragger
            accept=".txt,.docx"
            maxCount={1}
            fileList={fileList}
            beforeUpload={(f) => {
              // 客户端先拦一道体积：服务端上限 50MB，让浏览器直接把大文件挡掉，
              // 不必等上传完再收到 413（审查 P2：提示了上限却没有校验）
              if (f.size > MAX_UPLOAD_BYTES) {
                message.error(`文件 ${(f.size / 1024 / 1024).toFixed(1)}MB 超过上限 50MB，请先拆分或压缩`)
                return Upload.LIST_IGNORE
              }
              // antd 的 RcFile 本身就是 File 的子类，不需要双重断言
              setFile(f)
              setFileList([{ uid: f.uid, name: f.name }])
              return false // 阻止 antd 自动上传，改为点「开始导入」
            }}
            onRemove={() => {
              setFile(null)
              setFileList([])
            }}
          >
            <p className="ant-upload-drag-icon">
              <InboxOutlined />
            </p>
            <p className="ant-upload-text">点击或拖拽原著文件到这里</p>
            <p className="ant-upload-hint">.txt / .docx，单文件上限 50 MB</p>
          </Upload.Dragger>
          <Button type="primary" loading={importing} onClick={onImport} disabled={!file}>
            开始导入
          </Button>
        </Space>
      </Card>
    </Space>
  )
}
