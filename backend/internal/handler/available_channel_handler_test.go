//go:build unit

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserAvailableChannel_Unauthenticated401(t *testing.T) {
	// 没有 AuthSubject 注入时，handler 应返回 401 且不触达 service 依赖。
	gin.SetMode(gin.TestMode)
	h := &AvailableChannelHandler{} // nil services — 401 路径不会调用它们
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/channels/available", nil)

	h.List(c)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestFilterUserVisibleGroups_IntersectionOnly(t *testing.T) {
	// 渠道挂在 {g1, g2, g3}，用户只允许 {g1, g3} —— 响应必须仅含 g1/g3。
	groups := []service.AvailableGroupRef{
		{ID: 1, Name: "g1", Platform: "anthropic"},
		{ID: 2, Name: "g2", Platform: "anthropic"},
		{ID: 3, Name: "g3", Platform: "openai"},
	}
	allowed := map[int64]struct{}{1: {}, 3: {}}

	visible := filterUserVisibleGroups(groups, allowed)
	require.Len(t, visible, 2)
	ids := []int64{visible[0].ID, visible[1].ID}
	require.ElementsMatch(t, []int64{1, 3}, ids)
}

func TestToUserSupportedModels_FiltersByAllowedPlatforms(t *testing.T) {
	// 用户可访问分组只覆盖 anthropic；anthropic 平台的模型保留，openai 模型被剔除。
	src := []service.SupportedModel{
		{Name: "claude-sonnet-4-6", Platform: "anthropic", Pricing: nil},
		{Name: "gpt-4o", Platform: "openai", Pricing: nil},
	}
	allowed := map[string]struct{}{"anthropic": {}}
	out := toUserSupportedModels(src, allowed)
	require.Len(t, out, 1)
	require.Equal(t, "claude-sonnet-4-6", out[0].Name)
}

func TestToUserSupportedModels_NilAllowedPlatformsKeepsAll(t *testing.T) {
	// 显式传 nil allowedPlatforms 表示不做过滤。
	src := []service.SupportedModel{
		{Name: "a", Platform: "anthropic"},
		{Name: "b", Platform: "openai"},
	}
	require.Len(t, toUserSupportedModels(src, nil), 2)
}

func TestUserAvailableChannel_FieldWhitelist(t *testing.T) {
	// 通过序列化 userAvailableChannel 结构体验证响应形状：
	// 只有 name / description / platforms；不含管理端字段。
	row := userAvailableChannel{
		Name:        "ch",
		Description: "d",
		Platforms: []userChannelPlatformSection{
			{
				Platform:        "anthropic",
				Groups:          []userAvailableGroup{{ID: 1, Name: "g1", Platform: "anthropic"}},
				SupportedModels: []userSupportedModel{},
			},
		},
	}
	raw, err := json.Marshal(row)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))

	for _, key := range []string{"id", "status", "billing_model_source", "restrict_models"} {
		_, exists := decoded[key]
		require.Falsef(t, exists, "user DTO must not expose %q", key)
	}
	for _, key := range []string{"name", "description", "platforms"} {
		_, exists := decoded[key]
		require.Truef(t, exists, "user DTO must expose %q", key)
	}

	// 验证 section 的字段（platform / groups / supported_models）。
	rawSection, err := json.Marshal(row.Platforms[0])
	require.NoError(t, err)
	var sectionDecoded map[string]any
	require.NoError(t, json.Unmarshal(rawSection, &sectionDecoded))
	for _, key := range []string{"platform", "groups", "supported_models"} {
		_, exists := sectionDecoded[key]
		require.Truef(t, exists, "platform section must expose %q", key)
	}

	// Group DTO 暴露区分专属/公开、订阅类型、默认倍率所需的字段，
	// 前端据此渲染 GroupBadge 并与 API 密钥页保持一致的视觉。
	rawGroup, err := json.Marshal(row.Platforms[0].Groups[0])
	require.NoError(t, err)
	var groupDecoded map[string]any
	require.NoError(t, json.Unmarshal(rawGroup, &groupDecoded))
	for _, key := range []string{"id", "name", "platform", "subscription_type", "rate_multiplier", "is_exclusive"} {
		_, exists := groupDecoded[key]
		require.Truef(t, exists, "group DTO must expose %q", key)
	}

	// pricing interval 白名单：不应暴露 id / sort_order。
	pricing := toUserPricing(&service.ChannelModelPricing{
		BillingMode: service.BillingModeToken,
		Intervals: []service.PricingInterval{
			{ID: 7, MinTokens: 0, MaxTokens: nil, SortOrder: 3},
		},
	})
	require.NotNil(t, pricing)
	require.Len(t, pricing.Intervals, 1)
	rawIv, err := json.Marshal(pricing.Intervals[0])
	require.NoError(t, err)
	var ivDecoded map[string]any
	require.NoError(t, json.Unmarshal(rawIv, &ivDecoded))
	for _, key := range []string{"id", "pricing_id", "sort_order"} {
		_, exists := ivDecoded[key]
		require.Falsef(t, exists, "user pricing interval must not expose %q", key)
	}
}

func TestBuildGroupsInfo_CrossPlatformDesensitization(t *testing.T) {
	// 渠道同时挂在 anthropic 分组和 openai 分组上，并支持两个平台的模型。
	// 每个分组只应拿到与自身 platform 一致的模型。
	userGroups := []service.Group{
		{ID: 1, Name: "g-ant", Platform: "anthropic", RateMultiplier: 1},
		{ID: 2, Name: "g-oai", Platform: "openai", RateMultiplier: 2},
	}
	channels := []service.AvailableChannel{
		{
			Name:   "ch",
			Status: service.StatusActive,
			Groups: []service.AvailableGroupRef{
				{ID: 1, Platform: "anthropic"},
				{ID: 2, Platform: "openai"},
			},
			SupportedModels: []service.SupportedModel{
				{Name: "claude-sonnet-4-6", Platform: "anthropic"},
				{Name: "gpt-4o", Platform: "openai"},
			},
		},
	}

	out := buildGroupsInfo(userGroups, channels)
	require.Len(t, out, 2)
	// 排序：anthropic 在前。
	require.Equal(t, int64(1), out[0].ID)
	require.Len(t, out[0].Models, 1)
	require.Equal(t, "claude-sonnet-4-6", out[0].Models[0].Name)
	require.Equal(t, int64(2), out[1].ID)
	require.Len(t, out[1].Models, 1)
	require.Equal(t, "gpt-4o", out[1].Models[0].Name)
}

func TestBuildGroupsInfo_UnionDedupAcrossChannels(t *testing.T) {
	// 同一分组挂在两个渠道上，模型有重叠：结果应为并集且按模型名去重。
	userGroups := []service.Group{
		{ID: 1, Name: "g", Platform: "anthropic"},
	}
	channels := []service.AvailableChannel{
		{
			Name:            "ch1",
			Status:          service.StatusActive,
			Groups:          []service.AvailableGroupRef{{ID: 1, Platform: "anthropic"}},
			SupportedModels: []service.SupportedModel{{Name: "claude-a", Platform: "anthropic"}, {Name: "claude-b", Platform: "anthropic"}},
		},
		{
			Name:            "ch2",
			Status:          service.StatusActive,
			Groups:          []service.AvailableGroupRef{{ID: 1, Platform: "anthropic"}},
			SupportedModels: []service.SupportedModel{{Name: "claude-b", Platform: "anthropic"}, {Name: "claude-c", Platform: "anthropic"}},
		},
	}

	out := buildGroupsInfo(userGroups, channels)
	require.Len(t, out, 1)
	names := make([]string, len(out[0].Models))
	for i, m := range out[0].Models {
		names[i] = m.Name
	}
	require.Equal(t, []string{"claude-a", "claude-b", "claude-c"}, names)
}

func TestBuildGroupsInfo_SkipsInactiveChannelsAndDisallowedGroups(t *testing.T) {
	// 非 Active 渠道、以及不在 userGroups 中的分组，都不应贡献模型。
	userGroups := []service.Group{
		{ID: 1, Name: "g1", Platform: "anthropic"},
	}
	channels := []service.AvailableChannel{
		{ // 非 active —— 整渠道跳过
			Name:            "inactive",
			Status:          "inactive",
			Groups:          []service.AvailableGroupRef{{ID: 1, Platform: "anthropic"}},
			SupportedModels: []service.SupportedModel{{Name: "hidden", Platform: "anthropic"}},
		},
		{ // active 但只挂在用户无权访问的 g2 上
			Name:            "active",
			Status:          service.StatusActive,
			Groups:          []service.AvailableGroupRef{{ID: 2, Platform: "anthropic"}},
			SupportedModels: []service.SupportedModel{{Name: "alsohidden", Platform: "anthropic"}},
		},
	}

	out := buildGroupsInfo(userGroups, channels)
	require.Len(t, out, 1)
	require.Empty(t, out[0].Models)
}

func TestUserGroupInfo_FieldWhitelist(t *testing.T) {
	// 公开分组信息 DTO 暴露费率/平台/模型所需字段，不含管理端内部字段。
	row := userGroupInfo{
		ID: 1, Name: "g", Description: "d", Platform: "anthropic",
		SubscriptionType: "standard", RateMultiplier: 1.5, IsExclusive: false,
		Models: []userSupportedModel{{Name: "m", Platform: "anthropic"}},
	}
	raw, err := json.Marshal(row)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	for _, key := range []string{"id", "name", "description", "platform", "subscription_type", "rate_multiplier", "is_exclusive", "models"} {
		_, exists := decoded[key]
		require.Truef(t, exists, "group info DTO must expose %q", key)
	}
}

func TestBuildPlatformSections_GroupsByPlatform(t *testing.T) {
	// 一个渠道横跨 anthropic / openai / 空平台：应该生成 2 个 section，
	// 按 platform 字母序排序，各自 groups 和 supported_models 只含同平台条目。
	ch := service.AvailableChannel{
		Name: "ch",
		SupportedModels: []service.SupportedModel{
			{Name: "claude-sonnet-4-6", Platform: "anthropic"},
			{Name: "gpt-4o", Platform: "openai"},
		},
	}
	visible := []userAvailableGroup{
		{ID: 1, Name: "g-openai", Platform: "openai"},
		{ID: 2, Name: "g-ant", Platform: "anthropic"},
		{ID: 3, Name: "g-empty", Platform: ""},
	}
	sections := buildPlatformSections(ch, visible)
	require.Len(t, sections, 2)
	require.Equal(t, "anthropic", sections[0].Platform)
	require.Equal(t, "openai", sections[1].Platform)
	require.Len(t, sections[0].Groups, 1)
	require.Equal(t, int64(2), sections[0].Groups[0].ID)
	require.Len(t, sections[0].SupportedModels, 1)
	require.Equal(t, "claude-sonnet-4-6", sections[0].SupportedModels[0].Name)
}
