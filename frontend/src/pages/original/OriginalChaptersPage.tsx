import { Button, Card, Empty, Table, Typography, message } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'

import type { OriginalChapterBrief } from '../../api/types'
import { useOriginalStore } from '../../stores/originalStore'

export default function OriginalChaptersPage() {
  const navigate = useNavigate()
  const { workId, work, chapters, chaptersTotal, loading, loadChapters, loadWork } =
    useOriginalStore()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(50)

  useEffect(() => {
    if (!workId) return
    if (!work) void loadWork(workId).catch((err: Error) => message.error(err.message))
  }, [workId, work, loadWork])

  useEffect(() => {
    if (!workId) return
    void loadChapters(page, pageSize).catch((err: Error) => message.error(err.message))
  }, [workId, page, pageSize, loadChapters])

  if (!workId) {
    return (
      <Card title="章节目录">
        <Empty description={<span>还没有选择原著，<Link to="/original/overview">去选择</Link></span>} />
      </Card>
    )
  }

  const columns: ColumnsType<OriginalChapterBrief> = [
    { title: '章', dataIndex: 'chapter_no', key: 'chapter_no', width: 80 },
    { title: '标题', dataIndex: 'title', key: 'title' },
    {
      title: '字数',
      dataIndex: 'char_count',
      key: 'char_count',
      width: 120,
      render: (v: number) => v.toLocaleString('zh-CN'),
    },
    {
      title: '操作',
      key: 'actions',
      width: 100,
      render: (_, record) => (
        <Button type="link" size="small" onClick={() => navigate(`/original/chapters/${record.chapter_no}`)}>
          阅读
        </Button>
      ),
    },
  ]

  return (
    <Card
      title={work ? `《${work.title}》章节目录` : '章节目录'}
      extra={
        <Link to="/original/overview">
          <Button>返回总览</Button>
        </Link>
      }
    >
      {chaptersTotal === 0 && !loading ? (
        <Empty
          description={
            <Typography.Text type="secondary">
              还没有章节，去 <Link to="/original/overview">总览页导入原文</Link>
            </Typography.Text>
          }
        />
      ) : (
        <Table<OriginalChapterBrief>
          rowKey="chapter_no"
          loading={loading}
          dataSource={chapters}
          columns={columns}
          onRow={(record) => ({
            onClick: () => navigate(`/original/chapters/${record.chapter_no}`),
            style: { cursor: 'pointer' },
          })}
          pagination={{
            current: page,
            pageSize,
            total: chaptersTotal,
            showSizeChanger: true,
            showTotal: (t) => `共 ${t} 章`,
            onChange: (nextPage, nextSize) => {
              setPage(nextPage)
              setPageSize(nextSize)
            },
          }}
        />
      )}
    </Card>
  )
}
