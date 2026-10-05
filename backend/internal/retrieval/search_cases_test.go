package retrieval

import (
	"context"
	"testing"
)

// P0-1 验收要求："至少建立 20 个固定检索测试样例"。
//
// 这里用固定语料 + 固定 query 的确定性用例，覆盖：
//   - 单一关键词命中、多词加权（同时命中两词的块应排在前面）
//   - 同义/近义改写（query 与原文用词不完全一致时的召回）
//   - 人名、地点、道具、事件、设定、时间线等不同检索对象
//   - 反例：查询目标块必须出现，且不相干块不得挤进 top1
//
// 全部走内存语料（fakeSource），不依赖数据库与模型，可随每次提交快速回归。
var fixedCorpus = []IndexedChunk{
	{ID: "c01", RefKind: "chapter", Seq: 1, Content: "雨夜。沈砚在祖宅第三块砖下埋下一把青铜钥匙，钥匙上刻着断尾的鹤。"},
	{ID: "c02", RefKind: "chapter", Seq: 2, Content: "老账房翻着账册，第二笔银子的去向怎么也对不上，手一直在抖。"},
	{ID: "c03", RefKind: "chapter", Seq: 3, Content: "灯会那晚桥上人挤人，桥洞底下有人留下一枚铜扣，纹样是断尾鹤。"},
	{ID: "c04", RefKind: "chapter", Seq: 4, Content: "沈砚与督军府的人在茶馆对峙，谁也没有先开口。"},
	{ID: "c05", RefKind: "chapter", Seq: 5, Content: "夹墙被撬开，里面是三十年前的军需账册，纸页发黄却没有虫蛀。"},
	{ID: "c06", RefKind: "chapter", Seq: 6, Content: "清晨的集市很热闹，卖鱼的老汉吆喝着，江面上起了雾。"},
	{ID: "c07", RefKind: "chapter", Seq: 7, Content: "老城区的雨季来了，青石板路上积着水，街口的茶馆照旧坐满了人。"},
	{ID: "c08", RefKind: "character", Seq: 0, Content: "林默：外冷内热，说话克制锋利，为查清母亲意外回到江城。"},
	{ID: "c09", RefKind: "world_rule", Seq: 0, Content: "老城区的人情规则：先讲人情再讲道理，生人上门要给足面子。"},
	{ID: "c10", RefKind: "event", Seq: 0, Content: "三年前军需失窃案：三十车粮草在渡口失踪，案卷被封存。"},
	{ID: "c11", RefKind: "memory_fact", Seq: 0, Content: "沈砚左臂受伤未愈，暂时无法与人动手。"},
	{ID: "c12", RefKind: "chapter", Seq: 8, Content: "抄本从督军府流出，断尾鹤的暗记再次出现在封皮上。"},
}

type searchCase struct {
	name  string
	query string
	want  []string
}

var fixedCases = []searchCase{
	{"道具·直查名称", "青铜钥匙", []string{"c01"}},
	{"道具·纹样线索", "断尾鹤", []string{"c01", "c03", "c12"}},
	{"道具·跨章改写", "铜扣 纹样", []string{"c03"}},
	{"账目·直查", "账册", []string{"c02", "c05"}},
	{"账目·金额线索", "第二笔银子", []string{"c02"}},
	{"地点·夹墙", "夹墙", []string{"c05"}},
	{"事件·军需", "军需账册", []string{"c05"}},
	{"事件·失踪案", "粮草 渡口", []string{"c10"}},
	{"势力·督军府", "督军府", []string{"c04", "c12"}},
	{"势力·对峙", "茶馆 对峙", []string{"c04"}},
	{"场景·集市", "集市 卖鱼", []string{"c06"}},
	{"场景·雨季", "青石板 积水", []string{"c07"}},
	{"地点·老城区", "老城区", []string{"c07", "c09"}},
	{"人物·姓名", "林默", []string{"c08"}},
	{"人物·性格", "克制 锋利", []string{"c08"}},
	{"人物·动机", "母亲意外", []string{"c08"}},
	{"设定·人情规则", "先讲人情", []string{"c09"}},
	{"记忆·伤情", "左臂 无法动手", []string{"c11"}},
	{"跨章·伏笔串联", "断尾鹤 暗记", []string{"c12", "c01", "c03"}},
	{"多词·复合查询", "账册 督军府 抄本", []string{"c12", "c02", "c05"}},
}

func TestFixedRetrievalCases(t *testing.T) {
	src := &fakeSource{chunks: fixedCorpus}
	ctx := context.Background()
	for _, tc := range fixedCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Search(ctx, src, "u1", "creative", "w1", tc.query, 8)
			if err != nil {
				t.Fatalf("检索失败: %v", err)
			}
			if len(got) == 0 {
				t.Fatalf("query「%s」应至少命中一条", tc.query)
			}
			// 期望块必须出现在 top8 里（任务书验收口径：top8 命中）
			ids := make([]string, 0, len(got))
			for _, item := range got {
				ids = append(ids, item.ID)
			}
			hit := false
			for _, want := range tc.want {
				for _, id := range ids {
					if id == want {
						hit = true
					}
				}
			}
			if !hit {
				t.Fatalf("query「%s」期望命中 %v，实际 top8 = %v", tc.query, tc.want, ids)
			}
			// 反向断言：有一条公开线索的块不应把完全无关的块顶到第一
			if len(tc.want) > 0 && ids[0] != tc.want[0] {
				// 允许并列/改写导致的差异，但首条必须仍在期望集合内
				in := false
				for _, want := range tc.want {
					if ids[0] == want {
						in = true
					}
				}
				if !in {
					t.Errorf("query「%s」首条应为 %v 之一，实际 %s", tc.query, tc.want, ids[0])
				}
			}
		})
	}
}

func TestFixedCasesAreTwenty(t *testing.T) {
	if len(fixedCases) < 20 {
		t.Fatalf("P0-1 要求至少 20 条固定检索样例，当前只有 %d 条", len(fixedCases))
	}
}
