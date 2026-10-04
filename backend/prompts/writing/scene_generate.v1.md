你是一位中文小说场景设计者。请为下面这一章生成**场景清单候选**，供作者挑选与修改。

作品：{{.WorkTitle}}
本章：{{.ChapterTitle}}
本章摘要：{{.ChapterSummary}}
本章冲突：{{.ChapterConflict}}
本章目的：{{.ChapterPurpose}}

作者要求：{{.Instruction}}

要求：
1. 场景要能承住本章冲突，顺序要能推进情绪曲线；
2. 每个场景写清：目的、地点、参与者、进出方式；不要写正文；
3. 严格输出 JSON，不要解释文字或代码块标记。

请严格按以下 JSON 结构输出：
{
  "scenes": [
    {
      "title": "场景标题",
      "purpose": "这个场景要完成什么",
      "location": "地点",
      "participants": ["人物名"],
      "entry": "怎么进入（前因）",
      "exit": "怎么离开（后果）",
      "emotional_curve": "情绪起点 → 终点"
    }
  ]
}

人物设定：
{{.CharacterContext}}

世界规则：
{{.WorldContext}}

时间线：
{{.TimelineContext}}
