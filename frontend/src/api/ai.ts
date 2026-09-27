import { request } from './client'
import type {
  AnalysisStage,
  ListOf,
  ModelProvider,
  ModelProviderInput,
  ModelProviderTestResult,
  PromptMeta,
  Proposal,
  ProposalList,
  ProposalSummary,
  Task,
  TaskList,
} from './types'

/** 模型接入与 Prompt 模板 */
export const providerApi = {
  list() {
    return request<ListOf<ModelProvider>>('/model-providers')
  },
  create(input: ModelProviderInput): Promise<ModelProvider> {
    return request<ModelProvider>('/model-providers', { method: 'POST', body: JSON.stringify(input) })
  },
  update(id: string, input: ModelProviderInput): Promise<ModelProvider> {
    return request<ModelProvider>(`/model-providers/${id}`, { method: 'PUT', body: JSON.stringify(input) })
  },
  remove(id: string): Promise<{ deleted: boolean }> {
    return request<{ deleted: boolean }>(`/model-providers/${id}`, { method: 'DELETE' })
  },
  setDefault(id: string): Promise<ModelProvider> {
    return request<ModelProvider>(`/model-providers/${id}/default`, { method: 'POST' })
  },
  /** 真实调用一次上游验证配置 */
  test(id: string): Promise<ModelProviderTestResult> {
    return request<ModelProviderTestResult>(`/model-providers/${id}/test`, { method: 'POST' })
  },
  prompts() {
    return request<ListOf<PromptMeta>>('/prompts')
  },
}

/** 异步任务 */
export const taskApi = {
  list(params: { workId?: string; status?: string; page?: number; pageSize?: number } = {}) {
    const query = new URLSearchParams()
    if (params.workId) query.set('work_id', params.workId)
    if (params.status) query.set('status', params.status)
    query.set('page', String(params.page ?? 1))
    query.set('page_size', String(params.pageSize ?? 20))
    return request<TaskList>(`/tasks?${query.toString()}`)
  },
  get(id: string): Promise<Task> {
    return request<Task>(`/tasks/${id}`)
  },
  cancel(id: string): Promise<{ cancelled: boolean }> {
    return request<{ cancelled: boolean }>(`/tasks/${id}/cancel`, { method: 'POST' })
  },
  retry(id: string): Promise<Task> {
    return request<Task>(`/tasks/${id}/retry`, { method: 'POST' })
  },
}

/** AI 分析：触发阶段 + 审核提案 */
export const analysisApi = {
  enqueue(workId: string, stage: AnalysisStage): Promise<Task> {
    return request<Task>(`/original/${workId}/analysis`, {
      method: 'POST',
      body: JSON.stringify({ stage }),
    })
  },
  summary(workId: string): Promise<ProposalSummary> {
    return request<ProposalSummary>(`/original/${workId}/analysis/summary`)
  },
  listProposals(
    workId: string,
    params: { status?: string; stage?: string; entityType?: string; page?: number; pageSize?: number } = {},
  ) {
    const query = new URLSearchParams()
    if (params.status) query.set('status', params.status)
    if (params.stage) query.set('stage', params.stage)
    if (params.entityType) query.set('entity_type', params.entityType)
    query.set('page', String(params.page ?? 1))
    query.set('page_size', String(params.pageSize ?? 50))
    return request<ProposalList>(`/original/${workId}/proposals?${query.toString()}`)
  },
  getProposal(id: string): Promise<Proposal> {
    return request<Proposal>(`/proposals/${id}`)
  },
  /** 通过提案；payload 可传作者修改后的内容 */
  approve(id: string, payload?: Record<string, unknown>, note?: string): Promise<Proposal> {
    return request<Proposal>(`/proposals/${id}/approve`, {
      method: 'POST',
      body: JSON.stringify({ payload, note }),
    })
  },
  reject(id: string, note?: string): Promise<Proposal> {
    return request<Proposal>(`/proposals/${id}/reject`, {
      method: 'POST',
      body: JSON.stringify({ note }),
    })
  },
}
