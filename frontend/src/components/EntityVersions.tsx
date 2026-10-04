import { errorMessage } from '../api/client'
import { HistoryOutlined } from '@ant-design/icons'
import { Alert, Button, Drawer, Input, Modal, Space, Table, Tag, Tooltip, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useCallback, useState } from 'react'

import { versionApi } from '../api/versions'
import { ENTITY_VERSION_LABEL, type EntityVersion, type EntityVersionType } from '../api/types'
import VersionDiff from './VersionDiff'

interface Props {
  entityType: EntityVersionType
  entityId: string
  /** 恢复成功后通知外层刷新数据 */
  onRestored?: () => void
  /** 按钮文案，默认「版本」 */
  label?: string
  size?: 'small' | 'middle'
}

/**
 * 通用版本历史抽屉（人物 / 世界观 / 大纲共用）。
 *
 * 语义提醒直接写在界面上：快照是「某一刻长什么样」，恢复只回填快照里的字段，
 * 不会把快照之后新建的东西删掉 —— 免得作者以为回滚等于时光倒流。
 */
export default function EntityVersions({
  entityType,
  entityId,
  onRestored,
  label = '版本',
  size = 'small',
}: Props) {
  const [open, setOpen] = useState(false)
  const [items, setItems] = useState<EntityVersion[]>([])
  const [loading, setLoading] = useState(false)
  const [preview, setPreview] = useState<EntityVersion | null>(null)
  const [note, setNote] = useState('')
  const [busy, setBusy] = useState(false)
  const [diffFrom, setDiffFrom] = useState<number | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await versionApi.list(entityType, entityId)
      setItems(res.items)
    } catch (err) {
      message.error(errorMessage(err))
    } finally {
      setLoading(false)
    }
  }, [entityType, entityId])

  const openDrawer = () => {
    setOpen(true)
    void load()
  }

  const openPreview = async (no: number) => {
    try {
      setPreview(await versionApi.get(entityType, entityId, no))
    } catch (err) {
      message.error(errorMessage(err))
    }
  }

  const restore = async (no: number) => {
    setBusy(true)
    try {
      await versionApi.restore(entityType, entityId, no)
      message.success(`已恢复到 v${no}`)
      await load()
      onRestored?.()
    } catch (err) {
      message.error(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  const snapshot = async () => {
    setBusy(true)
    try {
      const res = await versionApi.snapshot(entityType, entityId, note)
      if (res.created === false) {
        message.info('内容与最新一版相同，未新建版本')
      } else {
        message.success('已存档')
      }
      setNote('')
      await load()
    } catch (err) {
      message.error(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  const columns: ColumnsType<EntityVersion> = [
    { title: '版本', dataIndex: 'version_no', width: 70, render: (no: number) => <Tag>v{no}</Tag> },
    {
      title: '说明',
      dataIndex: 'note',
      ellipsis: true,
      render: (v: string) => v || <Typography.Text type="secondary">—</Typography.Text>,
    },
    {
      title: '时间',
      dataIndex: 'created_at',
      width: 165,
      render: (v: string) => new Date(v).toLocaleString('zh-CN'),
    },
    {
      title: '操作',
      width: 210,
      render: (_, row) => (
        <Space size="small">
          <Button size="small" onClick={() => void openPreview(row.version_no)}>
            预览
          </Button>
          <Button size="small" onClick={() => setDiffFrom(row.version_no)}>
            比较
          </Button>
          <Button
            size="small"
            type="link"
            loading={busy}
            onClick={() => {
              Modal.confirm({
                title: `恢复到 v${row.version_no}？`,
                content: '只回填该快照里记录的字段；快照之后新建的内容不会被删除。',
                onOk: () => restore(row.version_no),
              })
            }}
          >
            恢复
          </Button>
        </Space>
      ),
    },
  ]

  return (
    <>
      <Tooltip title={`${ENTITY_VERSION_LABEL[entityType]}的版本历史`}>
        <Button size={size} icon={<HistoryOutlined />} onClick={openDrawer} aria-label={label}>
          {label}
        </Button>
      </Tooltip>

      <Drawer
        title={`${ENTITY_VERSION_LABEL[entityType]}版本历史`}
        width={760}
        open={open}
        onClose={() => setOpen(false)}
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="每次改动后自动存档；内容与上一版相同则不产生新版本"
          description="恢复只回填快照里记录的字段，快照之后新建的人物/规则/章节不会被删除。"
        />
        <Space style={{ marginBottom: 12 }}>
          <Input
            style={{ width: 260 }}
            placeholder="存档备注（可选）"
            value={note}
            onChange={(e) => setNote(e.target.value)}
          />
          <Button onClick={() => void snapshot()} loading={busy}>
            手动存档
          </Button>
        </Space>
        <Table
          rowKey="id"
          size="small"
          loading={loading}
          columns={columns}
          dataSource={items}
          pagination={false}
          locale={{ emptyText: '还没有版本（改动一次后自动生成）' }}
        />
      </Drawer>

      <Modal
        title={preview ? `v${preview.version_no} 快照` : '快照'}
        open={preview !== null}
        onCancel={() => setPreview(null)}
        footer={null}
        width={760}
      >
        <pre style={{ whiteSpace: 'pre-wrap', maxHeight: 480, overflow: 'auto', margin: 0, fontSize: 12 }}>
          {preview?.payload ? JSON.stringify(preview.payload, null, 2) : '（空）'}
        </pre>
      </Modal>

      <VersionDiff
        entityType={entityType}
        entityId={entityId}
        versions={items.map((v) => ({ version_no: v.version_no, note: v.note }))}
        open={diffFrom !== null}
        initialFrom={diffFrom ?? undefined}
        initialTo={0}
        onClose={() => setDiffFrom(null)}
      />
    </>
  )
}
