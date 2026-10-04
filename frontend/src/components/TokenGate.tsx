import { Alert, Button, Input, Modal, Space, Typography, message } from 'antd'
import { useCallback, useEffect, useState } from 'react'

import { request } from '../api/client'
import { TOKEN_REQUIRED_EVENT, getToken, setToken } from '../api/token'

/**
 * 访问令牌入口（P0 安全修复：接口不再匿名可访问）。
 *
 * 触发方式：任何请求收到 401 时由 client.ts 广播 TOKEN_REQUIRED_EVENT。
 * 不做"自动登录"这类花活：填对令牌 → 立刻用 /health 复验一次 → 刷新页面重新加载数据。
 */
export default function TokenGate() {
  const [open, setOpen] = useState(false)
  const [value, setValue] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    const onRequired = () => {
      setValue(getToken())
      setOpen(true)
    }
    window.addEventListener(TOKEN_REQUIRED_EVENT, onRequired)
    return () => window.removeEventListener(TOKEN_REQUIRED_EVENT, onRequired)
  }, [])

  const save = useCallback(async () => {
    const token = value.trim()
    setBusy(true)
    // 先写进去，request 会带上它
    setToken(token)
    try {
      await request<{ status: string }>('/health')
      message.success('访问令牌已保存')
      setOpen(false)
      window.location.reload()
    } catch {
      message.error('令牌无效：后端仍然拒绝了请求，请检查 ADMIN_TOKEN 是否一致')
    } finally {
      setBusy(false)
    }
  }, [value])

  return (
    <Modal
      title="需要访问令牌"
      open={open}
      onCancel={() => setOpen(false)}
      onOk={() => void save()}
      okText="保存并刷新"
      confirmLoading={busy}
      maskClosable={false}
    >
      <Space direction="vertical" size={12} style={{ width: '100%' }}>
        <Alert
          type="info"
          showIcon
          message="后端启用了访问令牌（ADMIN_TOKEN）"
          description="令牌在服务端的 .env 里配置，只存在你自己的浏览器里（localStorage），不会写进前端产物。"
        />
        <div>
          <Typography.Text type="secondary">访问令牌</Typography.Text>
          <Input.Password
            value={value}
            onChange={(e) => setValue(e.target.value)}
            placeholder="粘贴服务端 ADMIN_TOKEN"
            onPressEnter={() => void save()}
          />
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            生成方式（在服务器上执行）：openssl rand -hex 32
          </Typography.Text>
        </div>
        <Button
          size="small"
          onClick={() => {
            setToken('')
            setValue('')
            message.info('已清除本机保存的令牌')
          }}
        >
          清除本机令牌
        </Button>
      </Space>
    </Modal>
  )
}
