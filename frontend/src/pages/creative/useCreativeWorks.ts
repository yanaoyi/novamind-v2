import { useCallback, useEffect, useState } from 'react'

import { creativeApi } from '../../api/creative'
import type { CreativeWork } from '../../api/types'
import { useOriginalStore } from '../../stores/originalStore'

/**
 * 二创工作台共用的「当前二创作品」选择器：
 * 按当前原著列出它的全部二创作品，并维护选中项。
 */
export function useCreativeWorks() {
  const { workId: originalWorkId } = useOriginalStore()

  const [works, setWorks] = useState<CreativeWork[]>([])
  const [workId, setWorkId] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(
    async (targetId?: string) => {
      if (!originalWorkId) {
        setWorks([])
        setWorkId(null)
        return
      }
      setLoading(true)
      setError(null)
      try {
        const list = await creativeApi.listByOriginal(originalWorkId)
        setWorks(list.items)
        setWorkId((current) => {
          const wanted = targetId ?? current ?? list.items[0]?.id ?? null
          return list.items.some((w) => w.id === wanted) ? wanted : (list.items[0]?.id ?? null)
        })
      } catch (err) {
        setError((err as Error).message)
      } finally {
        setLoading(false)
      }
    },
    [originalWorkId],
  )

  useEffect(() => {
    void load()
  }, [load])

  const work = works.find((w) => w.id === workId) ?? null

  return { originalWorkId, works, work, workId, setWorkId, loading, error, reload: load }
}
