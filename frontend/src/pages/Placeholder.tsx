import { Card, Empty, Typography } from 'antd'

interface Props {
  title: string
  /** 该模块计划在哪个 Phase 实现 */
  phase?: string
}

/**
 * 占位页：路由先按 PRODUCT_SPEC §8 建全，功能按 Phase 逐步填充。
 * 保留占位是为了让导航结构从一开始就正确，避免后期大改路由。
 */
export default function Placeholder({ title, phase }: Props) {
  return (
    <Card title={title}>
      <Empty
        description={
          <Typography.Text type="secondary">
            {phase ? `该模块计划在 ${phase} 实现` : '该模块尚未实现'}
          </Typography.Text>
        }
      />
    </Card>
  )
}
