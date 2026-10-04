import type { OutlineNodeInput } from '../../api/types'

/**
 * 把 AI 生成的大纲候选（`outline_generate.v1.md` 约定的 JSON）转成提交给后端的节点树。
 *
 * 为什么要这一层转换：AI 的输出是「卷 → 节 → 章」三段式，而写入接口的契约是**嵌套即层级**。
 * 模型偶尔会省略「节」直接把章节挂在卷下面，这里补一个名为「正文」的节兜底，
 * 而不是把章节当成节写进去（那会让层级整体错位）。
 *
 * 转换只做形状整理，不改内容：标题/概要/三要素/人物都原样带过去，作者在界面上还能改。
 */

function asText(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

function asTextArray(value: unknown): string[] {
  if (Array.isArray(value)) {
    return value.map(asText).filter(Boolean)
  }
  const single = asText(value)
  return single ? [single] : []
}

function asRecords(value: unknown): Record<string, unknown>[] {
  if (!Array.isArray(value)) {
    return []
  }
  return value.filter((item): item is Record<string, unknown> => typeof item === 'object' && item !== null)
}

function toChapterNode(raw: Record<string, unknown>, fallbackTitle: string): OutlineNodeInput {
  return {
    title: asText(raw.title) || asText(raw.name) || fallbackTitle,
    summary: asText(raw.summary) || asText(raw.description),
    purpose: asText(raw.purpose),
    characters: asTextArray(raw.characters),
    location: asText(raw.location),
    conflict: asText(raw.conflict),
    outcome: asText(raw.outcome),
    children: [],
  }
}

/** 返回可提交的节点树；无法识别的输入返回空数组（调用方据此提示"模型没给出可用结构"）。 */
export function aiOutlineToNodeInputs(payload: unknown): OutlineNodeInput[] {
  if (!payload || typeof payload !== 'object') {
    return []
  }
  const obj = payload as Record<string, unknown>
  const rawVolumes = Array.isArray(obj.volumes)
    ? obj.volumes
    : Array.isArray(obj.outline)
      ? obj.outline
      : Array.isArray(obj.nodes)
        ? obj.nodes
        : []

  return asRecords(rawVolumes).map((volume, volumeIndex) => {
    const sections = asRecords(volume.sections)
    let children: OutlineNodeInput[] = []
    if (sections.length > 0) {
      children = sections.map((section, sectionIndex) => ({
        title: asText(section.title) || asText(section.name) || `第 ${sectionIndex + 1} 节`,
        summary: asText(section.summary) || asText(section.description),
        purpose: asText(section.purpose),
        location: asText(section.location),
        conflict: asText(section.conflict),
        outcome: asText(section.outcome),
        characters: asTextArray(section.characters),
        children: asRecords(section.chapters).map((chapter, chapterIndex) =>
          toChapterNode(chapter, `第 ${chapterIndex + 1} 章`),
        ),
      }))
    } else {
      const chapters = asRecords(volume.chapters)
      if (chapters.length > 0) {
        // 模型省略了「节」：补一层兜底，避免章节被当成节写进去
        children = [
          {
            title: '正文',
            summary: '',
            purpose: '',
            location: '',
            conflict: '',
            outcome: '',
            characters: [],
            children: chapters.map((chapter, chapterIndex) =>
              toChapterNode(chapter, `第 ${chapterIndex + 1} 章`),
            ),
          },
        ]
      }
    }
    return {
      title: asText(volume.title) || asText(volume.name) || `第 ${volumeIndex + 1} 卷`,
      summary: asText(volume.summary) || asText(volume.description),
      purpose: asText(volume.purpose),
      location: asText(volume.location),
      conflict: asText(volume.conflict),
      outcome: asText(volume.outcome),
      characters: asTextArray(volume.characters),
      children,
    }
  })
}

/** 统计候选树里的卷/节/章数量，用于界面上先给作者一个概览。 */
export function countOutlineInputs(nodes: OutlineNodeInput[]): { volumes: number; sections: number; chapters: number } {
  let volumes = 0
  let sections = 0
  let chapters = 0
  for (const volume of nodes) {
    volumes += 1
    for (const section of volume.children ?? []) {
      sections += 1
      chapters += (section.children ?? []).length
    }
  }
  return { volumes, sections, chapters }
}
