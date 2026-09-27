import { ArrowLeftOutlined } from '@ant-design/icons'
import { Button, Card, Empty, Space, Typography, message } from 'antd'
import { useEffect } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'

import { useOriginalStore } from '../../stores/originalStore'

export default function ChapterReaderPage() {
  const navigate = useNavigate()
  const params = useParams<{ no: string }>()
  const no = Number(params.no)

  const { workId, work, chapter, loading, loadWork, loadChapter } = useOriginalStore()

  useEffect(() => {
    if (!workId) return
    if (!work) void loadWork(workId).catch((err: Error) => message.error(err.message))
  }, [workId, work, loadWork])

  useEffect(() => {
    if (!workId || !Number.isFinite(no)) return
    void loadChapter(no).catch((err: Error) => message.error(err.message))
  }, [workId, no, loadChapter])

  if (!workId) {
    return (
      <Card title="章节阅读">
        <Empty description={<span>还没有选择原著，<Link to="/original/overview">去选择</Link></span>} />
      </Card>
    )
  }

  const total = work?.chapter_count ?? 0
  const hasPrev = no > 1
  const hasNext = total > 0 && no < total

  return (
    <Card
      title={chapter ? `第 ${chapter.chapter_no} 章 · ${chapter.title}` : '章节阅读'}
      extra={
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/original/chapters')}>
          返回目录
        </Button>
      }
      loading={loading && !chapter}
    >
      {chapter ? (
        <Space direction="vertical" size="middle" style={{ display: 'flex' }}>
          <Typography.Text type="secondary">
            共 {chapter.char_count.toLocaleString('zh-CN')} 字 · 原文位置 [{chapter.start_position}, {chapter.end_position})
          </Typography.Text>
          <Typography.Paragraph style={{ whiteSpace: 'pre-wrap', lineHeight: 1.9, marginBottom: 0 }}>
            {chapter.content}
          </Typography.Paragraph>
          <Space>
            <Button disabled={!hasPrev} onClick={() => navigate(`/original/chapters/${no - 1}`)}>
              上一章
            </Button>
            <Button
              type="primary"
              disabled={!hasNext}
              onClick={() => navigate(`/original/chapters/${no + 1}`)}
            >
              下一章
            </Button>
            <Typography.Text type="secondary">
              {total > 0 ? `第 ${no} / ${total} 章` : ''}
            </Typography.Text>
          </Space>
        </Space>
      ) : (
        <Empty description="章节不存在或还没加载" />
      )}
    </Card>
  )
}
