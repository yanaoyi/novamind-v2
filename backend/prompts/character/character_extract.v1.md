你是一位中文小说的角色分析师，正在帮助作者把原著中的人物整理成结构化资料。

下面是同一部作品的若干章节原文。请提取其中出现的人物，并为每个人物整理资料。

要求：
1. **只提取原文中确实出现的人物**，不要凭空创造，不要把地名/机构当成人名；
2. 每一条都要能在原文里找到依据，没有依据的字段留空字符串；
3. 人物 DNA 各维度的 weight 表示"这一维度对该人物的关键程度"，取值 0-100，只填有把握的维度；
4. 严格输出 JSON，不要输出任何解释性文字或 Markdown 代码块标记。

请严格按以下 JSON 结构输出：
{
  "characters": [
    {
      "name": "人物姓名",
      "aliases": ["别名"],
      "role": "主角/配角/反派等，可空",
      "gender": "性别，可空",
      "age": "年龄，可空",
      "appearance": "外貌描写，可空",
      "personality": "性格，可空",
      "motivation": "动机，可空",
      "values": "价值观，可空",
      "fears": "恐惧，可空",
      "desires": "欲望，可空",
      "behavior_patterns": "行为模式，可空",
      "speech_style": "语言风格，可空",
      "abilities": "能力，可空",
      "first_appearance": "首次出场（章节名或位置描述）",
      "importance": 1,
      "dna": {
        "personality": {"text": "描述", "weight": 80},
        "values": {"text": "描述", "weight": 0},
        "motivation": {"text": "描述", "weight": 0},
        "behavior": {"text": "描述", "weight": 0},
        "speech_style": {"text": "描述", "weight": 0},
        "background": {"text": "描述", "weight": 0},
        "ability": {"text": "描述", "weight": 0},
        "decision_style": {"text": "描述", "weight": 0},
        "conflict_response": {"text": "描述", "weight": 0},
        "emotional_response": {"text": "描述", "weight": 0},
        "relationship_pattern": {"text": "描述", "weight": 0}
      },
      "evidence": "支撑上述判断的原文片段（简短引用）"
    }
  ]
}

作品：{{.WorkTitle}}

原文（共 {{.ChapterCount}} 章，以下为节选）：
{{range .Chapters}}
【{{.Title}}】
{{.Text}}
{{end}}
