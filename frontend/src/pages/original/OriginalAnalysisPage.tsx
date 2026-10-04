import { errorMessage } from '../../api/client'
import { PlayCircleOutlined, ReloadOutlined } from '@ant-design/icons'
import {
  Alert,
  Button,
  Card,
  Descriptions,
  Empty,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Statistic,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useCallback, useEffect, useState } from 'react'
import { Link } from 'react-router-dom'

import { analysisApi } from '../../api/ai'
import {
  DNA_KEYS,
  DNA_LABELS,
  ENTITY_LABELS,
  PROPOSAL_STATUS_LABEL,
  STAGE_LABELS,
  type AnalysisStage,
  type DNAKey,
  type Proposal,
  type ProposalStatus,
  type ProposalSummary,
} from '../../api/types'
import { useOriginalStore } from '../../stores/originalStore'

const STAGES: AnalysisStage[] = ['chapter_summary', 'character_extract', 'world_extract', 'plot_extract']

const STATUS_COLOR: Record<ProposalStatus, string> = {
  PENDING: 'processing',
  APPROVED: 'success',
  REJECTED: 'default',
}

export default function OriginalAnalysisPage() {
  const { workId, work, loadWork } = useOriginalStore()
  const [proposals, setProposals] = useState<Proposal[]>([])
  const [total, setTotal] = useState(0)
  const [summary, setSummary] = useState<ProposalSummary>({ pending: 0, approved: 0, rejected: 0 })
  const [statusFilter, setStatusFilter] = useState<ProposalStatus | undefined>('PENDING')
  const [loading, setLoading] = useState(false)
  const [running, setRunning] = useState<AnalysisStage | null>(null)

  const [reviewing, setReviewing] = useState<Proposal | null>(null)
  const [payloadText, setPayloadText] = useState('')
  const [note, setNote] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const reload = useCallback(async () => {
    if (!workId) return
    setLoading(true)
    try {
      const [list, stat] = await Promise.all([
        analysisApi.listProposals(workId, { status: statusFilter, pageSize: 100 }),
        analysisApi.summary(workId),
      ])
      setProposals(list.items)
      setTotal(list.total)
      setSummary(stat)
    } catch (err) {
      message.error(errorMessage(err))
    } finally {
      setLoading(false)
    }
  }, [workId, statusFilter])

  useEffect(() => {
    if (!workId) return
    if (!work) void loadWork(workId).catch((err: Error) => message.error(err.message))
  }, [workId, work, loadWork])

  useEffect(() => {
    void reload()
  }, [reload])

  const trigger = async (stage: AnalysisStage) => {
    if (!workId) return
    setRunning(stage)
    try {
      const task = await analysisApi.enqueue(workId, stage)
      message.success(`已开始「${STAGE_LABELS[stage]}」，可在任务中心查看进度（任务 ${task.id.slice(0, 8)}）`)
    } catch (err) {
      message.error(errorMessage(err))
    } finally {
      setRunning(null)
    }
  }

  const openReview = (proposal: Proposal) => {
    setReviewing(proposal)
    setPayloadText(JSON.stringify(proposal.payload, null, 2))
    setNote('')
  }

  const approve = async () => {
    if (!reviewing) return
    let payload: Record<string, unknown>
    try {
      payload = JSON.parse(payloadText) as Record<string, unknown>
    } catch {
      message.error('内容不是合法 JSON，请检查后再通过')
      return
    }
    setSubmitting(true)
    try {
      const updated = await analysisApi.approve(reviewing.id, payload, note)
      message.success(updated.applied_id ? '已通过并写入原著' : '已通过')
      setReviewing(null)
      await reload()
    } catch (err) {
      message.error(errorMessage(err))
    } finally {
      setSubmitting(false)
    }
  }

  const reject = async () => {
    if (!reviewing) return
    setSubmitting(true)
    try {
      await analysisApi.reject(reviewing.id, note)
      message.success('已驳回')
      setReviewing(null)
      await reload()
    } catch (err) {
      message.error(errorMessage(err))
    } finally {
      setSubmitting(false)
    }
  }

  if (!workId) {
    return (
      <Card title="AI 分析">
        <Empty description={<span>还没有选择原著，<Link to="/original/overview">去选择</Link></span>} />
      </Card>
    )
  }

  const dnaPreview = (payload: Record<string, unknown>) => {
    const dna = payload['dna'] as Record<string, { text?: string; weight?: number }> | undefined
    if (!dna) return null
    const set = DNA_KEYS.map((key) => ({ key, ...(dna[key] ?? {}) })).filter((d) => (d.weight ?? 0) > 0)
    if (set.length === 0) return null
    return (
      <Space wrap size={4} style={{ marginTop: 8 }}>
        {set.map((d) => (
          <Tag key={d.key} color="blue">
            {DNA_LABELS[d.key as DNAKey]} {d.weight}%
          </Tag>
        ))}
      </Space>
    )
  }

  const columns: ColumnsType<Proposal> = [
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 100,
      render: (value: ProposalStatus) => <Tag color={STATUS_COLOR[value]}>{PROPOSAL_STATUS_LABEL[value]}</Tag>,
    },
    {
      title: '类型',
      dataIndex: 'entity_type',
      key: 'entity_type',
      width: 110,
      render: (value: keyof typeof ENTITY_LABELS) => <Tag>{ENTITY_LABELS[value] ?? value}</Tag>,
    },
    {
      title: '提案内容',
      key: 'title',
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{record.title || '（无标题）'}</Typography.Text>
          {record.evidence && (
            <Typography.Text type="secondary" style={{ fontSize: 12 }} ellipsis>
              依据：{record.evidence}
            </Typography.Text>
          )}
        </Space>
      ),
    },
    {
      title: '阶段',
      dataIndex: 'stage',
      key: 'stage',
      width: 130,
      render: (value: AnalysisStage) => <Typography.Text type="secondary">{STAGE_LABELS[value] ?? value}</Typography.Text>,
    },
    {
      title: '操作',
      key: 'actions',
      width: 110,
      render: (_, record) =>
        record.status === 'PENDING' ? (
          <Button type="link" size="small" onClick={() => openReview(record)}>
            审核
          </Button>
        ) : (
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {record.applied_id ? '已写入' : '—'}
          </Typography.Text>
        ),
    },
  ]

  return (
    <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
      <Card
        title={work ? `《${work.title}》AI 分析` : 'AI 分析'}
        extra={
          <Space>
            <Link to="/tasks">
              <Button>任务中心</Button>
            </Link>
            <Button icon={<ReloadOutlined />} onClick={() => void reload()}>
              刷新
            </Button>
          </Space>
        }
      >
        <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
          <Alert
            type="warning"
            showIcon
            message="AI 的产出不会直接改写原著"
            description="分析结果先进入「待审核」；你确认（可以改内容）之后，才会写进原著人物/世界观/事件等。"
          />
          <Space size="large">
            <Statistic title="待审核" value={summary.pending} />
            <Statistic title="已通过" value={summary.approved} />
            <Statistic title="已驳回" value={summary.rejected} />
          </Space>
          <Space wrap>
            {STAGES.map((stage) => (
              <Popconfirm
                key={stage}
                title={`触发「${STAGE_LABELS[stage]}」？`}
                description="会调用已配置的模型，可能消耗你的额度。"
                okText="开始"
                cancelText="取消"
                onConfirm={() => trigger(stage)}
              >
                <Button type="primary" ghost icon={<PlayCircleOutlined />} loading={running === stage}>
                  {STAGE_LABELS[stage]}
                </Button>
              </Popconfirm>
            ))}
          </Space>
        </Space>
      </Card>

      <Card
        title={`分析提案（${total}）`}
        extra={
          <Select
            allowClear
            placeholder="全部状态"
            style={{ width: 160 }}
            value={statusFilter}
            onChange={(value) => setStatusFilter(value)}
            options={(['PENDING', 'APPROVED', 'REJECTED'] as ProposalStatus[]).map((s) => ({
              value: s,
              label: PROPOSAL_STATUS_LABEL[s],
            }))}
          />
        }
      >
        {proposals.length === 0 && !loading ? (
          <Empty description="还没有提案。点上方的分析按钮，让 AI 先读一遍原著。" />
        ) : (
          <Table<Proposal>
            rowKey="id"
            loading={loading}
            dataSource={proposals}
            columns={columns}
            pagination={{ pageSize: 20 }}
          />
        )}
      </Card>

      <Modal
        open={reviewing !== null}
        title={reviewing ? `审核：${reviewing.title || ENTITY_LABELS[reviewing.entity_type]}` : ''}
        width={760}
        onCancel={() => setReviewing(null)}
        footer={[
          <Button key="reject" danger loading={submitting} onClick={reject}>
            驳回
          </Button>,
          <Button key="approve" type="primary" loading={submitting} onClick={approve}>
            通过并写入原著
          </Button>,
        ]}
        destroyOnHidden
      >
        {reviewing && (
          <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
            <Descriptions column={2} size="small">
              <Descriptions.Item label="实体类型">{ENTITY_LABELS[reviewing.entity_type]}</Descriptions.Item>
              <Descriptions.Item label="来源阶段">{STAGE_LABELS[reviewing.stage]}</Descriptions.Item>
            </Descriptions>
            {reviewing.evidence && (
              <Alert type="info" showIcon message="模型给出的原文依据" description={reviewing.evidence} />
            )}
            {dnaPreview(reviewing.payload)}
            <div>
              <Typography.Text strong>内容（可以改，改完再通过）</Typography.Text>
              <Input.TextArea
                rows={14}
                value={payloadText}
                onChange={(e) => setPayloadText(e.target.value)}
                style={{ fontFamily: 'monospace', marginTop: 8 }}
              />
            </div>
            <Input
              placeholder="审核备注（可空，例如：名字改成了更贴近原文的写法）"
              value={note}
              onChange={(e) => setNote(e.target.value)}
            />
          </Space>
        )}
      </Modal>
    </Space>
  )
}
