import { request } from './client'
import type {
  EventInput,
  EventList,
  ListOf,
  OriginalEvent,
  PlotArc,
  PlotArcInput,
  Timeline,
  TimelineItem,
} from './types'

/** 原著事件 / 时间线 / 剧情弧接口 */
export const eventApi = {
  list(workId: string, params: { page?: number; pageSize?: number; keyword?: string } = {}) {
    const query = new URLSearchParams()
    if (params.page) query.set('page', String(params.page))
    if (params.pageSize) query.set('page_size', String(params.pageSize))
    if (params.keyword) query.set('keyword', params.keyword)
    const qs = query.toString()
    return request<EventList>(`/original/${workId}/events${qs ? `?${qs}` : ''}`)
  },

  create(workId: string, input: EventInput): Promise<OriginalEvent> {
    return request<OriginalEvent>(`/original/${workId}/events`, {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },

  update(id: string, input: EventInput): Promise<OriginalEvent> {
    return request<OriginalEvent>(`/events/${id}`, { method: 'PUT', body: JSON.stringify(input) })
  },

  remove(id: string): Promise<{ deleted: boolean }> {
    return request<{ deleted: boolean }>(`/events/${id}`, { method: 'DELETE' })
  },

  getTimeline(workId: string): Promise<Timeline> {
    return request<Timeline>(`/original/${workId}/timeline`)
  },

  /** 整体替换时间线顺序（幂等）：传入顺序即最终顺序 */
  saveTimeline(workId: string, items: TimelineItem[]): Promise<Timeline> {
    return request<Timeline>(`/original/${workId}/timeline`, {
      method: 'PUT',
      body: JSON.stringify({ items }),
    })
  },

  listPlotArcs(workId: string) {
    return request<ListOf<PlotArc>>(`/original/${workId}/plot-arcs`)
  },
  createPlotArc(workId: string, input: PlotArcInput): Promise<PlotArc> {
    return request<PlotArc>(`/original/${workId}/plot-arcs`, {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },
  updatePlotArc(id: string, input: PlotArcInput): Promise<PlotArc> {
    return request<PlotArc>(`/plot-arcs/${id}`, { method: 'PUT', body: JSON.stringify(input) })
  },
  removePlotArc(id: string): Promise<{ deleted: boolean }> {
    return request<{ deleted: boolean }>(`/plot-arcs/${id}`, { method: 'DELETE' })
  },
}
