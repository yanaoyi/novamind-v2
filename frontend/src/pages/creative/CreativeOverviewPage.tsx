import { errorMessage } from '../../api/client'
import { BookOutlined, PlusOutlined, UploadOutlined } from '@ant-design/icons'
import {
  Alert,
  Button,
  Card,
  Col,
  Empty,
  Form,
  Input,
  List,
  Modal,
  Row,
  Space,
  Steps,
  Tag,
  Typography,
  Upload,
  message,
} from 'antd'
import type { UploadFile } from 'antd/es/upload/interface'
import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'

import { characterApi } from '../../api/characters'
import { createCreativeWork, creativeApi } from '../../api/creative'
import { originalApi } from '../../api/original'
import { projectApi } from '../../api/projects'
import { DNA_KEYS } from '../../api/types'
import { CREATIVE_STATUS_LABEL } from '../../api/types'
import { useOriginalStore } from '../../stores/originalStore'
import { useCreativeWorks } from './useCreativeWorks'

/** 全部维度按 100% 继承：派生人物与原著一致，作者之后再改 */
const FULL_WEIGHTS: Record<string, number> = Object.fromEntries(DNA_KEYS.map((k) => [k, 100]))

const ACCEPT = '.txt,.text,.md,.docx,.pdf'

/** 向导的四步（界面上让作者知道系统到底做了什么） */
const STEPS = ['建原著文章并导入原文', '建同人作品', '继承人物 DNA', '继承世界观']

/**
 * 二创 · 总览：同人坊的首页。
 *
 * 这里解决一个真实的别扭：想写同人，得先绕到「原著」那边导入书、配文章，再回二创派生作品。
 * 现在二创侧直接提供「导入一本书 → 一键开同人」：上传原文（TXT/MD/DOCX/PDF），
 * 系统在后台完成 原著文章 → 导入 → 同人作品 → 继承人物与世界观，
 * 作者落地就是「可写」状态。数据模型没变：原著仍是只读事实，同人作品归作者。
 */
export default function CreativeOverviewPage() {
  const navigate = useNavigate()
  const { workId: originalWorkId, work: originalWork, loadWork } = useOriginalStore()
  const { works, reload } = useCreativeWorks()

  const [fileList, setFileList] = useState<UploadFile[]>([])
  const [running, setRunning] = useState(false)
  const [stepIndex, setStepIndex] = useState(-1)
  const [importForm] = Form.useForm<{ title: string; author?: string }>()
  const [createOpen, setCreateOpen] = useState(false)
  const [createForm] = Form.useForm<{ title: string; description?: string }>()

  const applyWeightsFor = (characterId: string, workId: string) =>
    creativeApi.inheritCharacter(workId, { source_character_id: characterId, weights: FULL_WEIGHTS })

  /** 导入一本书并一键开同人 */
  const importAndCreate = async () => {
    const values = await importForm.validateFields()
    const file = fileList[0]?.originFileObj as File | undefined
    if (!file) {
      message.warning('请先选择原著文件（TXT / MD / DOCX / PDF）')
      return
    }
    setRunning(true)
    setStepIndex(0)
    try {
      // 1) 原著文章 + 导原文
      const originalProject = await projectApi.create({
        name: `${values.title}·原著`,
        type: 'ORIGINAL',
        description: `由二创入口导入：${file.name}`,
      })
      const original = await originalApi.create(originalProject.id, {
        title: values.title,
        author: values.author,
      })
      const result = await originalApi.importFile(original.id, file)
      setStepIndex(1)

      // 2) 同人作品（挂在 CREATIVE 文章下，从原著派生）
      const creativeProject = await projectApi.create({
        name: `${values.title}·同人`,
        type: 'CREATIVE',
        description: `基于《${values.title}》的同人作品`,
      })
      const creativeWork = await createCreativeWork(original.id, {
        project_id: creativeProject.id,
        title: `${values.title}·同人`,
        description: `基于《${values.title}》`,
      })

      // 3) 继承全部人物（权重 100%，即"先原样搬过来，再改"）
      const sources = await characterApi.list(original.id, { pageSize: 200 })
      let inherited = 0
      for (const c of sources.items) {
        try {
          await applyWeightsFor(c.id, creativeWork.id)
          inherited += 1
        } catch {
          // 单个人物继承失败不该毁掉整次导入（比如重名冲突），跳过并继续
        }
      }
      setStepIndex(2)

      // 4) 继承世界观（FULL：原著规则整套带过来）
      try {
        await creativeApi.inheritWorld(creativeWork.id, 'FULL')
      } catch {
        // 原著没建世界观时这里会失败，不算致命
      }
      setStepIndex(3)

      await loadWork(original.id)
      await reload(original.id)
      setFileList([])
      importForm.resetFields()
      message.success(
        `《${values.title}》已导入：切出 ${result.chapter_count} 章、${result.char_count.toLocaleString('zh-CN')} 字，` +
          `并继承 ${inherited} 个人物。可以开始写同人了。`,
      )
    } catch (err) {
      message.error(`导入失败：${errorMessage(err)}`)
    } finally {
      setRunning(false)
      setStepIndex(-1)
    }
  }

  /** 已有原著 → 直接开同人 */
  const createFromCurrent = async () => {
    const values = await createForm.validateFields()
    if (!originalWorkId) return
    try {
      const project = await projectApi.create({
        name: `${values.title}·同人`,
        type: 'CREATIVE',
        description: `基于《${originalWork?.title ?? values.title}》的同人作品`,
      })
      const creative = await createCreativeWork(originalWorkId, {
        project_id: project.id,
        title: values.title,
        description: values.description,
      })
      await reload(originalWorkId)
      setCreateOpen(false)
      createForm.resetFields()
      message.success('同人作品已创建，接着去「人物」把原著人物继承进来')
      navigate(`/creative/characters?work=${creative.id}`)
    } catch (err) {
      message.error(errorMessage(err))
    }
  }

  return (
    <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
      <Card
        title={
          <Space>
            <BookOutlined />
            导入一本书，开一部同人
          </Space>
        }
        extra={<Tag color="blue">支持 TXT / Markdown / DOCX / PDF</Tag>}
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="上传原文后，系统会自动完成四件事"
          description={
            <Steps
              size="small"
              direction="vertical"
              current={stepIndex < 0 ? 0 : stepIndex}
              status={running ? 'process' : 'wait'}
              items={STEPS.map((title, i) => ({
                title,
                status: running && stepIndex === i ? 'process' : stepIndex > i ? 'finish' : undefined,
              }))}
            />
          }
        />
        <Form form={importForm} layout="vertical">
          <Row gutter={12}>
            <Col span={10}>
              <Form.Item name="title" label="书名" rules={[{ required: true, message: '请输入书名' }]}>
                <Input placeholder="如：暗涌" />
              </Form.Item>
            </Col>
            <Col span={14}>
              <Form.Item name="author" label="作者（可选）">
                <Input placeholder="原作者" />
              </Form.Item>
            </Col>
          </Row>
        </Form>
        <Upload.Dragger
          accept={ACCEPT}
          maxCount={1}
          fileList={fileList}
          beforeUpload={(f) => {
            setFileList([{ uid: f.name, name: f.name, originFileObj: f } as UploadFile])
            // 用文件名当默认书名，省一步输入
            const guess = f.name.replace(/\.[^.]+$/, '')
            if (!importForm.getFieldValue('title')) importForm.setFieldsValue({ title: guess })
            return false
          }}
          onRemove={() => setFileList([])}
          disabled={running}
        >
          <p className="ant-upload-drag-icon">
            <UploadOutlined />
          </p>
          <p className="ant-upload-text">点击或把原著文件拖到这里</p>
          <p className="ant-upload-hint">
            PDF 需要带文本层（扫描件请先做 OCR）；重复导入同一本书会整体替换章节，不会重复累积。
          </p>
        </Upload.Dragger>
        <Space style={{ marginTop: 12 }}>
          <Button type="primary" loading={running} onClick={() => void importAndCreate()} disabled={running}>
            导入并创建同人作品
          </Button>
          <Typography.Text type="secondary">
            原著会被登记为「只读事实」，同人作品才能自由改；两者物理分离。
          </Typography.Text>
        </Space>
      </Card>

      <Card
        title={originalWork ? `当前原著的同人作品（${works.length}）` : '当前原著的同人作品'}
        extra={
          <Space>
            <Button
              icon={<PlusOutlined />}
              disabled={!originalWorkId}
              onClick={() => {
                createForm.setFieldsValue({ title: `${originalWork?.title ?? ''}·同人` })
                setCreateOpen(true)
              }}
            >
              从当前原著新建
            </Button>
            <Link to="/original/overview">
              <Button type="link">切换原著</Button>
            </Link>
          </Space>
        }
      >
        {!originalWorkId && (
          <Alert
            type="warning"
            showIcon
            message="还没有选中的原著"
            description="用上面的入口导入一本书，或在「原著 · 总览」里选一部已有的原著。"
          />
        )}
        {originalWorkId && (
          <>
            <Typography.Paragraph type="secondary">
              原著：{originalWork?.title ?? '（加载中）'}
            </Typography.Paragraph>
            <List
              dataSource={works}
              locale={{ emptyText: <Empty description="这部原著还没有同人作品" /> }}
              renderItem={(w) => (
                <List.Item
                  actions={[
                    <Link key="write" to="/creative/chapters">
                      去写作
                    </Link>,
                    <Link key="chars" to="/creative/characters">
                      人物
                    </Link>,
                    <Link key="world" to="/creative/world">
                      世界观
                    </Link>,
                  ]}
                >
                  <List.Item.Meta
                    title={w.title}
                    description={
                      <Space>
                        <Tag color={w.status === 'WRITING' ? 'green' : 'default'}>
                          {CREATIVE_STATUS_LABEL[w.status] ?? w.status}
                        </Tag>
                        <Typography.Text type="secondary">
                          创建于 {new Date(w.created_at).toLocaleDateString('zh-CN')}
                        </Typography.Text>
                      </Space>
                    }
                  />
                </List.Item>
              )}
            />
          </>
        )}
      </Card>

      <Modal
        title="从当前原著新建同人作品"
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        onOk={() => void createFromCurrent()}
      >
        <Form form={createForm} layout="vertical">
          <Form.Item name="title" label="作品标题" rules={[{ required: true, message: '请输入标题' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="description" label="简介">
            <Input.TextArea rows={3} />
          </Form.Item>
        </Form>
        <Alert
          type="info"
          showIcon
          message="创建后到「人物」里继承原著人物（可按维度调节继承比例），世界观在「世界观」标签继承。"
        />
      </Modal>
    </Space>
  )
}
