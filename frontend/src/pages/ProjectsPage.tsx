import { errorMessage } from '../api/client'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import {
  Button,
  Card,
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
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import type { Project, ProjectStatus, ProjectType } from '../api/types'
import { useProjectStore } from '../stores/projectStore'
import { useOriginalStore } from '../stores/originalStore'

const TYPE_LABEL: Record<ProjectType, string> = {
  ORIGINAL: '原著',
  CREATIVE: '二创',
}

const STATUS_LABEL: Record<ProjectStatus, string> = {
  DRAFT: '草稿',
  ACTIVE: '进行中',
  ARCHIVED: '已归档',
}

const STATUS_COLOR: Record<ProjectStatus, string> = {
  DRAFT: 'default',
  ACTIVE: 'green',
  ARCHIVED: 'orange',
}

interface FormValues {
  name: string
  description?: string
  type: ProjectType
  status?: ProjectStatus
}

export default function ProjectsPage() {
  const navigate = useNavigate()
  const {
    items,
    total,
    page,
    pageSize,
    loading,
    load,
    create,
    update,
    remove,
  } = useProjectStore()

  const [form] = Form.useForm<FormValues>()
  const setWorkId = useOriginalStore((s) => s.setWorkId)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<Project | null>(null)
  const [keyword, setKeyword] = useState('')
  const [typeFilter, setTypeFilter] = useState<ProjectType | undefined>(undefined)

  useEffect(() => {
    void load().catch((err: Error) => message.error(err.message))
  }, [load])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ type: 'CREATIVE' })
    setOpen(true)
  }

  const openEdit = (record: Project) => {
    setEditing(record)
    form.setFieldsValue({
      name: record.name,
      description: record.description,
      type: record.type,
      status: record.status,
    })
    setOpen(true)
  }

  const submit = async () => {
    const values = await form.validateFields()
    try {
      if (editing) {
        // 类型不可变更，因此更新时不提交 type
        await update(editing.id, {
          name: values.name,
          description: values.description,
          status: values.status,
        })
        message.success('已更新')
      } else {
        await create({
          name: values.name,
          description: values.description,
          type: values.type,
        })
        message.success('已创建')
      }
      setOpen(false)
    } catch (err) {
      message.error(errorMessage(err))
    }
  }

  const onDelete = async (record: Project) => {
    try {
      await remove(record.id)
      message.success('已删除')
    } catch (err) {
      message.error(errorMessage(err))
    }
  }

  /** 进入某工程的原著工作区（总览页会按需引导创建原著） */
  const enterOriginal = (record: Project) => {
    setWorkId(null) // 清掉上一部原著，交给总览页按目标工程重新定位
    navigate('/original/overview', { state: { projectId: record.id } })
  }

  const columns: ColumnsType<Project> = [
    { title: '名称', dataIndex: 'name', key: 'name', width: 220 },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 100,
      render: (value: ProjectType) => (
        <Tag color={value === 'ORIGINAL' ? 'blue' : 'purple'}>{TYPE_LABEL[value]}</Tag>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 110,
      render: (value: ProjectStatus) => <Tag color={STATUS_COLOR[value]}>{STATUS_LABEL[value]}</Tag>,
    },
    {
      title: '简介',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
      render: (value: string) => value || <Typography.Text type="secondary">—</Typography.Text>,
    },
    {
      title: '更新时间',
      dataIndex: 'updated_at',
      key: 'updated_at',
      width: 180,
      render: (value: string) => new Date(value).toLocaleString('zh-CN'),
    },
    {
      title: '操作',
      key: 'actions',
      width: 150,
      render: (_, record) => (
        <Space>
          {record.type === 'ORIGINAL' && (
            <Button type="link" size="small" onClick={() => enterOriginal(record)}>
              原著
            </Button>
          )}
          <Button type="link" size="small" onClick={() => openEdit(record)}>
            编辑
          </Button>
          <Popconfirm
            title="确认删除该工程？"
            description="为软删除，数据不会物理丢失"
            okText="确认删除"
            cancelText="取消"
            onConfirm={() => onDelete(record)}
          >
            <Button type="link" size="small" danger>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <Card
      title="工程管理"
      extra={
        <Space>
          <Input.Search
            allowClear
            placeholder="按名称搜索"
            style={{ width: 220 }}
            onSearch={(value) => {
              setKeyword(value)
              void load({ keyword: value, page: 1 })
            }}
          />
          <Select
            allowClear
            placeholder="全部类型"
            style={{ width: 140 }}
            value={typeFilter}
            options={[
              { value: 'ORIGINAL', label: '原著' },
              { value: 'CREATIVE', label: '二创' },
            ]}
            onChange={(value) => {
              setTypeFilter(value)
              void load({ type: value, page: 1 })
            }}
          />
          <Button icon={<ReloadOutlined />} onClick={() => void load()}>
            刷新
          </Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新建工程
          </Button>
        </Space>
      }
    >
      <Table<Project>
        rowKey="id"
        loading={loading}
        dataSource={items}
        columns={columns}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          showTotal: (t) => `共 ${t} 个工程`,
          onChange: (nextPage, nextSize) =>
            void load({ page: nextPage, page_size: nextSize, keyword, type: typeFilter }),
        }}
      />

      <Modal
        open={open}
        title={editing ? '编辑工程' : '新建工程'}
        okText={editing ? '保存' : '创建'}
        cancelText="取消"
        onOk={submit}
        onCancel={() => setOpen(false)}
        destroyOnHidden
      >
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item
            name="name"
            label="名称"
            rules={[{ required: true, message: '请输入工程名称' }]}
          >
            <Input placeholder="例如：人间真相" maxLength={200} />
          </Form.Item>

          <Form.Item
            name="type"
            label="类型"
            rules={[{ required: true, message: '请选择类型' }]}
            extra={editing ? '类型创建后不可变更' : '原著=导入并分析的原作；二创=基于原著的新作品'}
          >
            <Select
              disabled={Boolean(editing)}
              options={[
                { value: 'ORIGINAL', label: '原著' },
                { value: 'CREATIVE', label: '二创' },
              ]}
            />
          </Form.Item>

          {editing && (
            <Form.Item name="status" label="状态">
              <Select
                options={[
                  { value: 'DRAFT', label: '草稿' },
                  { value: 'ACTIVE', label: '进行中' },
                  { value: 'ARCHIVED', label: '已归档' },
                ]}
              />
            </Form.Item>
          )}

          <Form.Item name="description" label="简介">
            <Input.TextArea rows={3} placeholder="一句话说明这部作品" />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  )
}
