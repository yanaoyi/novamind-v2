import { Button, Modal, Select, Space, Switch, Table, Tag, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useCallback, useEffect, useState } from 'react'

import { versionApi } from '../api/versions'
import type {
  VersionCompare,
  VersionCompareEntityType,
  VersionTextLine,
  VersionValueChange,
} from '../api/types'

interface VersionOption {
  version_no: number
  note?: string
}

interface Props {
  entityType: VersionCompareEntityType
  entityId: string
  /** 可选版本（不含「当前」，组件自己加） */
  versions: VersionOption[]
  open: boolean
  /** 打开时的默认基准版本 */
  initialFrom?: number
  /** 打开时的默认对比版本，默认 0（当前） */
  initialTo?: number
  onClose: () => void
}

/**
 * 版本比较弹窗（规格书 §59 的「比较」）。
 *
 * 结构化实体（人物 / 世界观 / 大纲）走字段级 diff；
 * 章节走正文行级 diff（相同行默认折叠，可以切「只看差异」）。
 */
export default function VersionDiff({
  entityType,
  entityId,
  versions,
  open,
  initialFrom,
  initialTo = 0,
  onClose,
}: Props) {
  const [from, setFrom] = useState<number>(initialFrom ?? 0)
  const [to, setTo] = useState<number>(initialTo)
  const [result, setResult] = useState<VersionCompare | null>(null)
  const [loading, setLoading] = useState(false)
  const [onlyDiff, setOnlyDiff] = useState(true)

  useEffect(() => {
    if (open) {
      setFrom(initialFrom ?? 0)
      setTo(initialTo)
      setResult(null)
    }
  }, [open, initialFrom, initialTo])

  const options = [
    { value: 0, label: '当前状态' },
    ...versions.map((v) => ({
      value: v.version_no,
      label: `v${v.version_no}${v.note ? ` · ${v.note}` : ''}`,
    })),
  ]

  const run = useCallback(async () => {
    if (from === to) {
      message.warning('两个版本相同，换一个再比')
      return
    }
    setLoading(true)
    try {
      setResult(await versionApi.compare({ entityType, entityId, from, to }))
    } catch (err) {
      message.error((err as Error).message)
    } finally {
      setLoading(false)
    }
  }, [entityType, entityId, from, to])

  const changeColumns: ColumnsType<VersionValueChange> = [
    {
      title: '字段',
      dataIndex: 'path',
      width: 220,
      render: (v: string) => <Typography.Text code>{v}</Typography.Text>,
    },
    {
      title: '变化',
      dataIndex: 'kind',
      width: 80,
      render: (v: VersionValueChange['kind']) => (
        <Tag color={v === 'added' ? 'green' : v === 'removed' ? 'red' : 'orange'}>
          {v === 'added' ? '新增' : v === 'removed' ? '删除' : '修改'}
        </Tag>
      ),
    },
    {
      title: '变更前',
      dataIndex: 'before',
      render: (v: unknown) => renderValue(v),
    },
    {
      title: '变更后',
      dataIndex: 'after',
      render: (v: unknown) => renderValue(v),
    },
  ]

  const visibleLines = result?.lines.filter((l) => (onlyDiff ? l.kind !== 'same' : true)) ?? []

  return (
    <Modal
      title="版本比较"
      open={open}
      onCancel={onClose}
      footer={null}
      width={920}
      destroyOnClose
    >
      <Space style={{ marginBottom: 12 }} wrap>
        <Select style={{ width: 220 }} value={from} options={options} onChange={setFrom} />
        <Typography.Text type="secondary">对比</Typography.Text>
        <Select style={{ width: 220 }} value={to} options={options} onChange={setTo} />
        <Button type="primary" loading={loading} onClick={() => void run()}>
          比较
        </Button>
      </Space>

      {result && (
        <>
          <Space style={{ marginBottom: 12 }} wrap>
            <Tag color="green">新增 {result.summary.added}</Tag>
            <Tag color="red">删除 {result.summary.removed}</Tag>
            <Tag color="orange">修改 {result.summary.changed}</Tag>
            <Tag>未变 {result.summary.same}</Tag>
            {result.from_note && (
              <Typography.Text type="secondary">基准：{result.from_note}</Typography.Text>
            )}
            {result.to_note && (
              <Typography.Text type="secondary">对比：{result.to_note}</Typography.Text>
            )}
          </Space>

          {result.changes.length > 0 && (
            <Table
              rowKey={(row) => `${row.path}-${row.kind}`}
              size="small"
              style={{ marginBottom: 12 }}
              columns={changeColumns}
              dataSource={result.changes}
              pagination={result.changes.length > 20 ? { pageSize: 20 } : false}
            />
          )}

          {result.lines.length > 0 && (
            <>
              <Space style={{ marginBottom: 8 }}>
                <Typography.Text strong>正文差异</Typography.Text>
                <Switch checked={onlyDiff} onChange={setOnlyDiff} checkedChildren="只看差异" unCheckedChildren="全部行" />
              </Space>
              <div style={{ maxHeight: 420, overflow: 'auto', border: '1px solid #f0f0f0', padding: 8 }}>
                {visibleLines.length === 0 ? (
                  <Typography.Text type="secondary">正文没有差异</Typography.Text>
                ) : (
                  visibleLines.map((line, idx) => <DiffLine key={idx} line={line} />)
                )}
              </div>
            </>
          )}

          {result.changes.length === 0 && result.lines.length === 0 && (
            <Typography.Text type="secondary">两个版本没有差异。</Typography.Text>
          )}
        </>
      )}
    </Modal>
  )
}

function DiffLine({ line }: { line: VersionTextLine }) {
  const color = line.kind === 'added' ? '#f6ffed' : line.kind === 'removed' ? '#fff1f0' : 'transparent'
  const sign = line.kind === 'added' ? '+' : line.kind === 'removed' ? '-' : ' '
  const no = line.new_no ?? line.old_no ?? ''
  return (
    <div style={{ background: color, whiteSpace: 'pre-wrap', fontFamily: 'monospace', fontSize: 12 }}>
      <span style={{ display: 'inline-block', width: 48, color: '#999' }}>{no}</span>
      <span>{sign} {line.text || ' '}</span>
    </div>
  )
}

function renderValue(value: unknown) {
  if (value === null || value === undefined) {
    return <Typography.Text type="secondary">（无）</Typography.Text>
  }
  const text = typeof value === 'string' ? value : JSON.stringify(value)
  return (
    <Typography.Text style={{ whiteSpace: 'pre-wrap' }}>
      {text.length > 300 ? `${text.slice(0, 300)}…` : text}
    </Typography.Text>
  )
}
