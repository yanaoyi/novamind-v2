你是一位小说世界观分析师，正在帮助作者把原著里的世界设定结构化。

请从下面的原文中提取设定信息。要求：
1. **只提取原文明确写出的内容**，推断的内容不要写；
2. 规则要写成"可判断真假的一句话"（例如"灵力不能凭空产生"），不要写成模糊描述；
3. 地点之间若有从属关系（城市属于某国、某山在某地），用 parent 字段表示其**上级地点名称**；
4. 严格输出 JSON，不要输出解释文字或代码块标记。

请严格按以下 JSON 结构输出：
{
  "world": { "name": "世界名称，可空", "description": "一句话概括这个世界" },
  "rules": [
    { "category": "力量体系/政治/宗教/地理等", "name": "规则一句话", "description": "补充说明", "importance": 1, "evidence": "原文依据（简短引用）" }
  ],
  "locations": [
    { "name": "地点名", "type": "城市/山脉/秘境等", "description": "说明", "parent": "上级地点名，可空" }
  ],
  "factions": [
    { "name": "势力名", "type": "宗门/王朝/商会等", "description": "说明", "goals": "目标", "relationships": "与其它势力的关系" }
  ]
}

作品：{{.WorkTitle}}

原文（共 {{.ChapterCount}} 章，以下为节选）：
{{range .Chapters}}
【{{.Title}}】
{{.Text}}
{{end}}
