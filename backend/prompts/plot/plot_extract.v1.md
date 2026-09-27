你是一位剧情结构分析师，正在帮助作者梳理原著的事件与剧情线。

请从下面的原文中提取关键事件，并归纳剧情线。要求：
1. **只提取原文确实发生的事件**，不要把叙述性背景当成事件；
2. 每个事件写清"发生了什么"与"造成了什么后果"；
3. 事件按时间先后排列（time_order 从 10 开始，每项 +10）；
4. 剧情线 type 只能取：main / subplot / character_arc / relationship_arc / world_arc；
5. 严格输出 JSON，不要输出解释文字或代码块标记。

请严格按以下 JSON 结构输出：
{
  "events": [
    {
      "title": "事件标题",
      "description": "发生了什么",
      "chapter_no": 1,
      "time_order": 10,
      "participants": ["参与人物姓名"],
      "location": "发生地点名，可空",
      "consequences": "造成了什么后果",
      "importance": 3,
      "evidence": "原文依据（简短引用）"
    }
  ],
  "plot_arcs": [
    {
      "type": "main",
      "title": "剧情线标题",
      "summary": "这条线在讲什么",
      "start_event": "起始事件标题",
      "end_event": "结束事件标题"
    }
  ]
}

作品：{{.WorkTitle}}

原文（共 {{.ChapterCount}} 章，以下为节选）：
{{range .Chapters}}
【{{.Title}}】
{{.Text}}
{{end}}
