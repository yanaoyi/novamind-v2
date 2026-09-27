import { PlusOutlined, ReloadOutlined, ThunderboltOutlined } from '@ant-design/icons'
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Slider,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useCallback, useEffect, useState } from 'react'

import { providerApi } from '../api/ai'
import { PROVIDER_LABELS, type ModelProvider, type ModelProviderInput, type PromptMeta, type ProviderType } from '../api/types'

interface FormValues extends Omit<ModelProviderInput, 'provider' | 'purpose'> {
  provider: ProviderType
}

export default function SettingsPage() {
  const [providers, setProviders] = useState<ModelProvider[]>([])
  const [prompts, setPrompts] = useState<PromptMeta[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<ModelProvider | null>(null)
  const [testing, setTesting] = useState<string | null>(null)
  const [form] = Form.useForm<FormValues>()

  const reload = useCallback(async () => {
    setLoading(true)
    try {
      const [providerList, promptList] = await Promise.all([providerApi.list(), providerApi.prompts()])
      setProviders(providerList.items)
      setPrompts(promptList.items)
    } catch (err) {
      message.error((err as Error).message)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void reload()
  }, [reload])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({
      provider: 'OPENAI_COMPATIBLE',
      temperature: 0.7,
      max_tokens: 4096,
      timeout_sec: 120,
      enabled: true,
      is_default: false,
    })
    setOpen(true)
  }

  const openEdit = (record: ModelProvider) => {
    setEditing(record)
    form.setFieldsValue({
      name: record.name,
      provider: record.provider,
      api_base: record.api_base,
      api_key: '',
      model_name: record.model_name,
      temperature: record.temperature,
      max_tokens: record.max_tokens,
      timeout_sec: record.timeout_sec,
      enabled: record.enabled,
      is_default: record.is_default,
      notes: record.notes,
    })
    setOpen(true)
  }

  const submit = async () => {
    const values = await form.validateFields()
    try {
      if (editing) {
        await providerApi.update(editing.id, values)
        message.success('配置已更新')
      } else {
        await providerApi.create(values)
        message.success('配置已创建')
      }
      setOpen(false)
      await reload()
    } catch (err) {
      message.error((err as Error).message)
    }
  }

  const test = async (record: ModelProvider) => {
    setTesting(record.id)
    try {
      const result = await providerApi.test(record.id)
      if (result.ok) {
        message.success(`连通成功：${result.model} 回复「${result.reply}」（${result.latency_ms}ms / ${result.total_tokens} tokens）`)
      } else {
        Modal.error({
          title: '连通失败',
          content: <Typography.Text code>{result.error_message}</Typography.Text>,
        })
      }
    } catch (err) {
      message.error((err as Error).message)
    } finally {
      setTesting(null)
    }
  }

  const columns: ColumnsType<ModelProvider> = [
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      render: (value: string, record) => (
        <Space>
          <Typography.Text strong>{value}</Typography.Text>
          {record.is_default && <Tag color="green">默认</Tag>}
          {!record.enabled && <Tag>已停用</Tag>}
        </Space>
      ),
    },
    { title: '模型', dataIndex: 'model_name', key: 'model_name', width: 180 },
    {
      title: '接口地址',
      dataIndex: 'api_base',
      key: 'api_base',
      ellipsis: true,
      render: (value: string) => <Typography.Text type="secondary">{value}</Typography.Text>,
    },
    {
      title: '密钥',
      dataIndex: 'has_api_key',
      key: 'has_api_key',
      width: 90,
      render: (has: boolean) => (has ? <Tag color="blue">已保存</Tag> : <Tag color="red">未填</Tag>),
    },
    {
      title: '操作',
      key: 'actions',
      width: 300,
      render: (_, record) => (
        <Space>
          <Button
            type="link"
            size="small"
            icon={<ThunderboltOutlined />}
            loading={testing === record.id}
            onClick={() => test(record)}
          >
            测试
          </Button>
          {!record.is_default && (
            <Button
              type="link"
              size="small"
              onClick={async () => {
                try {
                  await providerApi.setDefault(record.id)
                  message.success('已设为默认模型')
                  await reload()
                } catch (err) {
                  message.error((err as Error).message)
                }
              }}
            >
              设为默认
            </Button>
          )}
          <Button type="link" size="small" onClick={() => openEdit(record)}>
            编辑
          </Button>
          <Popconfirm
            title={`删除「${record.name}」？`}
            okText="确认删除"
            cancelText="取消"
            onConfirm={async () => {
              try {
                await providerApi.remove(record.id)
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

  return (
    <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
      <Card
        title="模型接入"
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={() => void reload()}>
              刷新
            </Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>
              新增模型配置
            </Button>
          </Space>
        }
      >
        <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
          <Alert
            type="info"
            showIcon
            message="密钥只以密文保存在服务器，接口不会返回它"
            description="密钥用 AES-256-GCM 加密，主密钥来自服务器的 NOVAMIND_SECRET。换主密钥会导致已保存的密钥无法解密，只能重新填写。"
          />
          {providers.length === 0 && !loading ? (
            <Typography.Text type="secondary">
              还没有配置模型。AI 分析（人物提取、世界观提取等）需要一个可用的对话模型。
            </Typography.Text>
          ) : (
            <Table<ModelProvider> rowKey="id" loading={loading} dataSource={providers} columns={columns} pagination={false} />
          )}
        </Space>
      </Card>

      <Card title={`Prompt 模板（${prompts.length}）`} size="small">
        <Typography.Paragraph type="secondary">
          模板随程序分发（文件名即版本：<Typography.Text code>{'<name>.<version>.md'}</Typography.Text>）。
          AI 每次调用都会记录用的是哪个模板与版本，便于回溯"同一本书为什么两次结果不同"。
        </Typography.Paragraph>
        <Space wrap>
          {prompts.map((p) => (
            <Tag key={p.path}>
              {p.name} <Typography.Text type="secondary">{p.version}</Typography.Text>
            </Tag>
          ))}
        </Space>
      </Card>

      <Modal
        open={open}
        title={editing ? `编辑模型配置：${editing.name}` : '新增模型配置'}
        width={640}
        okText="保存"
        cancelText="取消"
        onOk={submit}
        onCancel={() => setOpen(false)}
        destroyOnHidden
      >
        <Form form={form} name="provider" layout="vertical" preserve={false}>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="例如：DeepSeek 主力" maxLength={120} />
          </Form.Item>
          <Form.Item name="provider" label="提供商类型" rules={[{ required: true }]}>
            <Select
              options={(Object.keys(PROVIDER_LABELS) as ProviderType[]).map((t) => ({
                value: t,
                label: PROVIDER_LABELS[t],
              }))}
            />
          </Form.Item>
          <Form.Item
            name="api_base"
            label="接口地址"
            rules={[{ required: true, message: '请输入接口地址' }]}
            extra="OpenAI 兼容填到 /v1 为止，例如 https://api.deepseek.com/v1"
          >
            <Input placeholder="https://api.deepseek.com/v1" />
          </Form.Item>
          <Form.Item
            name="model_name"
            label="模型名"
            rules={[{ required: true, message: '请输入模型名' }]}
          >
            <Input placeholder="deepseek-chat" />
          </Form.Item>
          <Form.Item
            name="api_key"
            label="API Key"
            extra={editing ? '留空表示保持原密钥不变' : '提交后用 AES-256-GCM 加密存储'}
          >
            <Input.Password placeholder={editing ? '（不修改）' : 'sk-…'} autoComplete="new-password" />
          </Form.Item>
          <Space size="middle" style={{ display: 'flex' }}>
            <Form.Item name="temperature" label="温度" style={{ minWidth: 220 }}>
              <Slider min={0} max={2} step={0.1} />
            </Form.Item>
            <Form.Item name="max_tokens" label="最大 token" style={{ minWidth: 140 }}>
              <InputNumber min={64} max={200000} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="timeout_sec" label="超时（秒）" style={{ minWidth: 140 }}>
              <InputNumber min={5} max={600} style={{ width: '100%' }} />
            </Form.Item>
          </Space>
          <Space size="large">
            <Form.Item name="enabled" label="启用" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Form.Item name="is_default" label="设为默认" valuePropName="checked">
              <Switch />
            </Form.Item>
          </Space>
          <Form.Item name="notes" label="备注">
            <Input.TextArea rows={2} />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
