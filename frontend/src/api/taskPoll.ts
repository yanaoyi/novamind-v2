import { taskApi } from './ai'
import type { Task } from './types'

const TERMINAL = ['COMPLETED', 'FAILED', 'CANCELLED']

/** 轮询任务直到终态（异步任务统一用这个，避免各处写法不一致） */
export async function pollTask(
  taskId: string,
  onTick?: (task: Task) => void,
  timeoutMs = 180_000,
): Promise<Task> {
  const startedAt = Date.now()
  for (;;) {
    const task = await taskApi.get(taskId)
    onTick?.(task)
    if (TERMINAL.includes(task.status)) return task
    if (Date.now() - startedAt > timeoutMs) {
      throw new Error('任务长时间未完成，可到「任务中心」查看进度')
    }
    await new Promise((resolve) => setTimeout(resolve, 1500))
  }
}
