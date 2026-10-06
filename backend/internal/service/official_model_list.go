package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// 官方模型名单（muqian 2026-10-06 定）：建渠道时上游名单里目录没有的模型，联网查 LiteLLM 公开价格表判断是不是
// 官方模型 ID——是就能带着官方价加进目录，不是就在承接关系上做别名映射（上游模型名）。
//
// 只认目录白名单里官方厂商自己的条目（openrouter、azure、bedrock 这类转售不算）：键去掉厂商前缀后按小写精确比对，
// 不做去日期、去前缀之类的识别——「anthropic/claude-…」这种写法不是官方 ID，该映射到目录里已有的模型。
// 2026-10-06 实测这份表 4472 条、138 个提供方；它会删掉下线的模型（claude-opus-4-1、grok-4 已不在），
// 也可能还没收录刚发布的，所以查不到只说明「表里没有」，最后由管理员判断。
//
// 「是不是官方 ID」与「读不读得出价格」分开：官方厂商的 ID 都算官方；价格能按内置价格表同一口径读出就带上，
// 读不出（tiered_pricing 分档价、按张 / 按秒之类）留空让管理员填。2026-10-06 实测官方条目里 dashscope 20 条、
// openai 62 条没有单一 token 价（qwen3-max 就是三档分档价）。
//
// 名单在内存里缓存 officialModelListTTL；拉取失败时沿用上一份，一份都没有就报不可用。

const (
	officialModelListURL = "https://raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json"
	// officialModelListTTL 拍的：LiteLLM 一天合并多次，新模型当天能查到就够。
	officialModelListTTL = 6 * time.Hour
	// officialModelListRetryAfter 拉取失败后多久再试（拍的），免得每个请求都去撞一次超时。
	officialModelListRetryAfter = time.Minute
	officialModelListTimeout    = 20 * time.Second
	// officialModelListMaxBytes 2026-10-06 实测 3.0 MB，留足余量。
	officialModelListMaxBytes = 32 << 20
)

// officialModelProviders LiteLLM 里算官方厂商的提供方 → 写进目录条目的厂商串。只认目录白名单里的 11 家
// （catalogVendorAllowlist，muqian 2026-10-06）；与内置目录同一套写法：智谱在 LiteLLM 里叫 zai、目录里叫 zhipu，
// 小米 MiMo 叫 xiaomi_mimo、目录里叫 xiaomi。
var officialModelProviders = map[string]string{
	"openai":      "openai",
	"anthropic":   "anthropic",
	"gemini":      "gemini",
	"xai":         "xai",
	"deepseek":    "deepseek",
	"dashscope":   "dashscope",
	"zai":         "zhipu",
	"volcengine":  "volcengine",
	"moonshot":    "moonshot",
	"minimax":     "minimax",
	"xiaomi_mimo": "xiaomi",
}

// ErrOfficialModelListUnavailable 联网名单一份都没拉到过。
var ErrOfficialModelListUnavailable = errors.New("official model list unavailable")

// officialModel 名单里的一个官方模型：Priced 为 false 时 Entry 只有模型 ID 与厂商。
type officialModel struct {
	Entry  ModelCatalogEntry
	Priced bool
}

// officialModelList 联网拉取并缓存的官方模型名单。
type officialModelList struct {
	url    string
	client *http.Client
	now    func() time.Time

	mu         sync.Mutex
	index      map[string]officialModel // 小写模型 ID → 官方模型
	fetchedAt  time.Time
	retryAfter time.Time
	flight     singleflight.Group
}

func newOfficialModelList() *officialModelList {
	return &officialModelList{
		url:    officialModelListURL,
		client: &http.Client{Timeout: officialModelListTimeout},
		now:    time.Now,
	}
}

// lookup 按官方模型 ID 精确查（不分大小写），条目的模型 ID 用查询时的写法。名单不可用时返回 ErrOfficialModelListUnavailable。
func (l *officialModelList) lookup(ctx context.Context, modelID string) (officialModel, bool, error) {
	index, err := l.load(ctx)
	if err != nil {
		return officialModel{}, false, err
	}
	model, ok := index[strings.ToLower(strings.TrimSpace(modelID))]
	if ok {
		model.Entry.ModelID = strings.TrimSpace(modelID)
	}
	return model, ok, nil
}

// load 返回当前名单：没过期直接用；过期了重新拉，拉不到沿用上一份。并发的请求只拉一次。
func (l *officialModelList) load(ctx context.Context) (map[string]officialModel, error) {
	l.mu.Lock()
	now := l.now()
	index, fetchedAt, retryAfter := l.index, l.fetchedAt, l.retryAfter
	l.mu.Unlock()
	if index != nil && now.Sub(fetchedAt) < officialModelListTTL {
		return index, nil
	}
	if now.Before(retryAfter) {
		if index != nil {
			return index, nil
		}
		return nil, ErrOfficialModelListUnavailable
	}

	// 拉取不跟着单个请求取消：别的请求可能在等同一次拉取
	result, err, _ := l.flight.Do("load", func() (any, error) {
		fetched, err := l.fetch(context.WithoutCancel(ctx))
		l.mu.Lock()
		defer l.mu.Unlock()
		if err != nil {
			l.retryAfter = l.now().Add(officialModelListRetryAfter)
			return nil, err
		}
		l.index, l.fetchedAt = fetched, l.now()
		return fetched, nil
	})
	if err != nil {
		if index != nil {
			return index, nil
		}
		return nil, fmt.Errorf("%w: %v", ErrOfficialModelListUnavailable, err)
	}
	return result.(map[string]officialModel), nil
}

func (l *officialModelList) fetch(ctx context.Context) (map[string]officialModel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("official model list returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, officialModelListMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > officialModelListMaxBytes {
		return nil, fmt.Errorf("official model list exceeds %d bytes", officialModelListMaxBytes)
	}
	return parseOfficialModelList(body)
}

// parseOfficialModelList 从 LiteLLM 价格表里挑出官方厂商的条目，按去掉厂商前缀后的模型 ID 建索引。
// 价格换算复用内置价格表的解析与播种（同一份格式、同一套口径）。同一个模型 ID 有不带前缀与带前缀两条时
// 用不带前缀的（OpenAI / Anthropic 的写法）。
func parseOfficialModelList(body []byte) (map[string]officialModel, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse official model list: %w", err)
	}

	type candidate struct {
		modelID  string
		vendor   string
		prefixed bool
	}
	filtered := make(map[string]json.RawMessage)
	candidates := make(map[string]candidate)
	for key, value := range raw {
		if key == "sample_spec" { // LiteLLM 的字段说明，不是模型
			continue
		}
		var head struct {
			Provider string `json:"litellm_provider"`
		}
		if json.Unmarshal(value, &head) != nil {
			continue
		}
		provider := strings.ToLower(strings.TrimSpace(head.Provider))
		vendor, ok := officialModelProviders[provider]
		if !ok {
			continue
		}
		modelID, prefixed := key, false
		if i := strings.Index(key, "/"); i >= 0 {
			// 只去掉和提供方同名的前缀；其余带斜杠的（gpt-image 的「质量/尺寸/模型」之类）不是模型 ID
			if !strings.EqualFold(key[:i], provider) {
				continue
			}
			modelID, prefixed = key[i+1:], true
		}
		if modelID = strings.TrimSpace(modelID); modelID == "" || strings.Contains(modelID, "/") {
			continue
		}
		filtered[key] = value
		candidates[key] = candidate{modelID: modelID, vendor: vendor, prefixed: prefixed}
	}

	filteredBody, err := json.Marshal(filtered)
	if err != nil {
		return nil, err
	}
	pricings, err := (&PricingService{}).parsePricingData(filteredBody)
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(candidates))
	for key := range candidates {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	type slot struct {
		model    officialModel
		prefixed bool
	}
	slots := make(map[string]slot, len(keys))
	for _, key := range keys {
		c := candidates[key]
		model := officialModel{Entry: ModelCatalogEntry{ModelID: c.modelID, BillingMode: BillingModeToken, Status: ModelCatalogStatusUnlisted}}
		if pricing := pricings[key]; pricing != nil {
			entry := seedEntryFromLiteLLM(c.modelID, pricing)
			// 与播种同一口径：按 token 计费却没有 token 价的不带价（会按 $0 计费）
			if !(entry.BillingMode == BillingModeToken && pricing.TokenPricingAbsent) {
				model = officialModel{Entry: entry, Priced: true}
			}
		}
		// 给管理员新建条目用：不是播种来的
		model.Entry.Vendor, model.Entry.ManagedBy = c.vendor, ""
		// 同一个 ID 多条时：有价的优先，同样有价 / 没价时不带前缀的优先
		lower := strings.ToLower(c.modelID)
		if existing, seen := slots[lower]; seen {
			better := (model.Priced && !existing.model.Priced) ||
				(model.Priced == existing.model.Priced && existing.prefixed && !c.prefixed)
			if !better {
				continue
			}
		}
		slots[lower] = slot{model: model, prefixed: c.prefixed}
	}
	index := make(map[string]officialModel, len(slots))
	for key, s := range slots {
		index[key] = s.model
	}
	return index, nil
}

// OfficialModelMatch 一个上游模型 ID 是不是官方 ID；是的话带上厂商与官方价（建目录条目时预填）。
// Priced 为 false 表示名单里有这个 ID 但读不出价格（分档价等），价格要管理员填。
type OfficialModelMatch struct {
	ModelID  string             `json:"model_id"`
	Official bool               `json:"official"`
	Priced   bool               `json:"priced"`
	Entry    *ModelCatalogEntry `json:"entry,omitempty"`
}

// OfficialModelLookupResult 批量查询的结果。
type OfficialModelLookupResult struct {
	// Available 联网名单可用；false 时 Official 一律为 false，界面提示查不到、由管理员判断。
	Available bool                 `json:"available"`
	Models    []OfficialModelMatch `json:"models"`
}

// LookupOfficialModels 逐个判断上游模型 ID 是不是官方 ID（按联网名单精确比对），顺序与去重后的输入一致。
func (s *ModelCatalogService) LookupOfficialModels(ctx context.Context, modelIDs []string) OfficialModelLookupResult {
	result := OfficialModelLookupResult{Models: []OfficialModelMatch{}}
	seen := make(map[string]struct{}, len(modelIDs))
	ids := make([]string, 0, len(modelIDs))
	for _, raw := range modelIDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, dup := seen[strings.ToLower(id)]; dup {
			continue
		}
		seen[strings.ToLower(id)] = struct{}{}
		ids = append(ids, id)
	}
	var index map[string]officialModel
	if s != nil && s.officialModels != nil {
		if loaded, err := s.officialModels.load(ctx); err == nil {
			index, result.Available = loaded, true
		}
	}
	for _, id := range ids {
		match := OfficialModelMatch{ModelID: id}
		if model, ok := index[strings.ToLower(id)]; ok {
			entry := model.Entry
			entry.ModelID = id
			match.Official, match.Priced, match.Entry = true, model.Priced, &entry
		}
		result.Models = append(result.Models, match)
	}
	return result
}

// LookupPriceEntry 按模型 ID 带出建议条目给「添加模型」预填：先查内置价格资料（可识别日期后缀等写法），
// 查不到再按官方 ID 查联网名单。联网名单不可用时只按内置资料。
func (s *ModelCatalogService) LookupPriceEntry(ctx context.Context, modelID string) (ModelCatalogEntry, bool) {
	if entry, ok := s.LookupPriceFileEntry(modelID); ok {
		return entry, true
	}
	if s == nil || s.officialModels == nil || strings.TrimSpace(modelID) == "" {
		return ModelCatalogEntry{}, false
	}
	model, ok, err := s.officialModels.lookup(ctx, modelID)
	if err != nil || !ok || !model.Priced {
		return ModelCatalogEntry{}, false
	}
	return model.Entry, true
}
