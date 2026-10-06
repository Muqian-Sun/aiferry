package service

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

const hotReloadCatalogModelJSON = `"remote-model": {"litellm_provider": "openai", "mode": "chat",
		"input_cost_per_token": 1e-06, "output_cost_per_token": 2e-06}`

func hotReloadModelJSON(name string, input, output float64) string {
	return `"` + name + `": {"litellm_provider": "openai", "mode": "chat",
		"input_cost_per_token": ` + formatFloat(input) + `, "output_cost_per_token": ` + formatFloat(output) + `}`
}

// hotReloadBuiltinJSON 拼出内置价格文件正文：固定带 remote-model，再加上给定条目。
func hotReloadBuiltinJSON(extra ...string) string {
	body := `{` + hotReloadCatalogModelJSON
	for _, e := range extra {
		body += `,` + e
	}
	return body + `}`
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// newHotReloadPricingService 写好内置价格文件与 override 文件并完成首次加载。
// overrideJSON 传空串表示不配置 override。
func newHotReloadPricingService(t *testing.T, builtinJSON, overrideJSON string) *PricingService {
	t.Helper()
	dir := t.TempDir()
	svc := &PricingService{cfg: &config.Config{}}
	svc.cfg.Pricing.FallbackFile = filepath.Join(dir, "model_prices.json")
	require.NoError(t, os.WriteFile(svc.cfg.Pricing.FallbackFile, []byte(builtinJSON), 0644))
	if overrideJSON != "" {
		svc.cfg.Pricing.OverrideFile = filepath.Join(dir, "overrides.json")
		require.NoError(t, os.WriteFile(svc.cfg.Pricing.OverrideFile, []byte(overrideJSON), 0644))
	}
	require.NoError(t, svc.reloadPricingFiles())
	return svc
}

func TestPricingCustomFilesFingerprint(t *testing.T) {
	svc := &PricingService{cfg: &config.Config{}}
	require.Empty(t, svc.customPricingFilesFingerprint(), "未配置文件返回空串")

	dir := t.TempDir()
	svc.cfg.Pricing.FallbackFile = filepath.Join(dir, "fallback.json")
	missing := svc.customPricingFilesFingerprint()
	require.NotEmpty(t, missing)
	require.Equal(t, missing, svc.customPricingFilesFingerprint(), "同一状态下指纹稳定")

	require.NoError(t, os.WriteFile(svc.cfg.Pricing.FallbackFile, []byte(`{}`), 0644))
	present := svc.customPricingFilesFingerprint()
	require.NotEqual(t, missing, present, "文件出现即视为变化")

	svc.cfg.Pricing.OverrideFile = filepath.Join(dir, "overrides.json")
	require.NoError(t, os.WriteFile(svc.cfg.Pricing.OverrideFile, []byte(`{}`), 0644))
	require.NotEqual(t, present, svc.customPricingFilesFingerprint(), "override 内容参与指纹")
}

func TestPricingHotReload_BuiltinFileChangeRebuilds(t *testing.T) {
	svc := newHotReloadPricingService(t, hotReloadBuiltinJSON(hotReloadModelJSON("custom-a", 4e-6, 8e-6)), "")
	require.InDelta(t, 4e-6, svc.pricingData["custom-a"].InputCostPerToken, 1e-12)
	require.Nil(t, svc.pricingData["custom-b"])
	require.NotEmpty(t, svc.customFilesHash)

	require.NoError(t, os.WriteFile(svc.cfg.Pricing.FallbackFile, []byte(hotReloadBuiltinJSON(
		hotReloadModelJSON("custom-a", 5e-6, 8e-6),
		hotReloadModelJSON("custom-b", 1e-6, 3e-6))), 0644))
	svc.reloadIfCustomFilesChanged()

	require.InDelta(t, 5e-6, svc.pricingData["custom-a"].InputCostPerToken, 1e-12, "改价即时生效")
	require.NotNil(t, svc.pricingData["custom-b"], "新模型即时并入")
	require.InDelta(t, 1e-6, svc.pricingData["remote-model"].InputCostPerToken, 1e-12, "未改动的条目不受影响")
	require.Equal(t, svc.customPricingFilesFingerprint(), svc.customFilesHash)
}

func TestPricingHotReload_UnchangedFilesSkipRebuild(t *testing.T) {
	svc := newHotReloadPricingService(t,
		hotReloadBuiltinJSON(hotReloadModelJSON("custom-a", 4e-6, 8e-6)),
		`{"remote-model": {"input_cost_per_token": 7e-06}}`)
	svc.pricingData["sentinel"] = &LiteLLMModelPricing{}

	svc.reloadIfCustomFilesChanged()

	require.Contains(t, svc.pricingData, "sentinel", "指纹未变时不得重建")
}

func TestPricingHotReload_OverrideChangePatchesCatalogAndAddsModels(t *testing.T) {
	svc := newHotReloadPricingService(t, hotReloadBuiltinJSON(), `{"remote-model": {"input_cost_per_token": 7e-06}}`)
	require.InDelta(t, 7e-6, svc.pricingData["remote-model"].InputCostPerToken, 1e-12)

	require.NoError(t, os.WriteFile(svc.cfg.Pricing.OverrideFile, []byte(`{
		"remote-model": {"input_cost_per_token": 9e-06},
		`+hotReloadModelJSON("override-new-model", 5e-6, 1e-5)+`}`), 0644))
	svc.reloadIfCustomFilesChanged()

	require.InDelta(t, 9e-6, svc.pricingData["remote-model"].InputCostPerToken, 1e-12)
	require.InDelta(t, 2e-6, svc.pricingData["remote-model"].OutputCostPerToken, 1e-12, "未覆盖字段保持目录值")
	require.NotNil(t, svc.pricingData["override-new-model"])

	// 清空补丁：目录条目回到原价，补丁新增的模型随之消失。
	require.NoError(t, os.WriteFile(svc.cfg.Pricing.OverrideFile, []byte(`{}`), 0644))
	svc.reloadIfCustomFilesChanged()

	require.InDelta(t, 1e-6, svc.pricingData["remote-model"].InputCostPerToken, 1e-12)
	require.Nil(t, svc.pricingData["override-new-model"])
}

func TestPricingHotReload_InvalidFileKeepsCurrentDataUntilFixed(t *testing.T) {
	svc := newHotReloadPricingService(t, hotReloadBuiltinJSON(hotReloadModelJSON("custom-a", 4e-6, 8e-6)), "")
	before := svc.customFilesHash

	require.NoError(t, os.WriteFile(svc.cfg.Pricing.FallbackFile, []byte(`{"custom-a": {"input_cost_per_token": `), 0644))
	svc.reloadIfCustomFilesChanged()
	require.InDelta(t, 4e-6, svc.pricingData["custom-a"].InputCostPerToken, 1e-12, "半写文件不得替换数据")
	require.Equal(t, before, svc.customFilesHash, "指纹不更新，下一轮继续尝试")

	require.NoError(t, os.WriteFile(svc.cfg.Pricing.FallbackFile, []byte(hotReloadBuiltinJSON(hotReloadModelJSON("custom-a", 6e-6, 8e-6))), 0644))
	svc.reloadIfCustomFilesChanged()
	require.InDelta(t, 6e-6, svc.pricingData["custom-a"].InputCostPerToken, 1e-12, "文件修好后正常重建")
	require.NotEqual(t, before, svc.customFilesHash)
}

// 删除 override 等于清空该层，在下一轮比对时生效，且缺失状态被记录、之后不会每轮重建。
// 内置价格文件是价格的唯一来源：它暂时缺失时保留当前价格（不会清空成 0），文件回来后恢复。
func TestPricingHotReload_DeletedFileHandling(t *testing.T) {
	svc := newHotReloadPricingService(t,
		hotReloadBuiltinJSON(hotReloadModelJSON("custom-a", 4e-6, 8e-6)),
		`{"remote-model": {"input_cost_per_token": 7e-06}}`)
	require.NotNil(t, svc.pricingData["custom-a"])
	require.InDelta(t, 7e-6, svc.pricingData["remote-model"].InputCostPerToken, 1e-12)

	require.NoError(t, os.Remove(svc.cfg.Pricing.OverrideFile))
	svc.reloadIfCustomFilesChanged()
	require.InDelta(t, 1e-6, svc.pricingData["remote-model"].InputCostPerToken, 1e-12, "删除 override 后目录条目回到原价")
	require.NotNil(t, svc.pricingData["custom-a"], "内置价格不受影响")

	svc.pricingData["sentinel"] = &LiteLLMModelPricing{}
	svc.reloadIfCustomFilesChanged()
	require.Contains(t, svc.pricingData, "sentinel", "缺失状态已记录，不得每轮重建")
	delete(svc.pricingData, "sentinel")

	loaded := svc.customFilesHash
	require.NoError(t, os.Remove(svc.cfg.Pricing.FallbackFile))
	svc.reloadIfCustomFilesChanged()
	require.NotNil(t, svc.pricingData["custom-a"], "内置价格文件缺失时保留当前价格")
	require.InDelta(t, 1e-6, svc.pricingData["remote-model"].InputCostPerToken, 1e-12)
	require.Equal(t, loaded, svc.customFilesHash, "加载失败不更新指纹，下一轮继续尝试")

	require.NoError(t, os.WriteFile(svc.cfg.Pricing.FallbackFile, []byte(hotReloadBuiltinJSON(hotReloadModelJSON("custom-a", 6e-6, 8e-6))), 0644))
	svc.reloadIfCustomFilesChanged()
	require.InDelta(t, 6e-6, svc.pricingData["custom-a"].InputCostPerToken, 1e-12, "文件回来即按新内容加载")
}

// 配置了价格文件时调度器要运行，否则文件改动无人比对；Stop 后退出。
func TestPricingSchedulerStartsWhenPricingFileConfigured(t *testing.T) {
	svc := NewPricingService(&config.Config{Pricing: config.PricingConfig{
		FallbackFile: filepath.Join(t.TempDir(), "fallback.json"),
	}})

	svc.startUpdateScheduler()
	done := make(chan struct{})
	go func() {
		svc.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("pricing file watch must keep the scheduler running")
	case <-time.After(50 * time.Millisecond):
	}

	svc.Stop()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scheduler must exit after Stop")
	}
}

// 价格数据真的换过才通知回调：文件未变的空轮不触发；文件变了重载一次触发一次。
func TestPricingUpdateHook_FiresOnlyWhenDataChanged(t *testing.T) {
	svc := newHotReloadPricingService(t, hotReloadBuiltinJSON(hotReloadModelJSON("custom-a", 4e-6, 8e-6)), "")
	fired := 0
	svc.OnPricingUpdated(func() { fired++ })

	svc.runScheduledUpdate()
	require.Equal(t, 0, fired, "an idle round must not notify")

	require.NoError(t, os.WriteFile(svc.cfg.Pricing.FallbackFile, []byte(hotReloadBuiltinJSON(hotReloadModelJSON("custom-a", 5e-6, 8e-6))), 0644))
	svc.runScheduledUpdate()
	require.Equal(t, 1, fired, "a reload that replaces the data must notify")

	svc.runScheduledUpdate()
	require.Equal(t, 1, fired, "the next idle round must not notify again")
}

// 装配层把「价格文件更新 → 目录重播」挂上：价格文件改价重载后，seed 条目要跟上新价。
func TestProvideModelCatalogService_ReseedsAfterPricingUpdate(t *testing.T) {
	pricing := newHotReloadPricingService(t, hotReloadBuiltinJSON(), "")
	bs := &BillingService{fallbackPrices: map[string]*ModelPricing{}}
	repo := &stubModelCatalogRepo{}

	catalog := ProvideModelCatalogService(repo, nil, pricing, bs)
	_, err := catalog.Seed(context.Background())
	require.NoError(t, err)
	before := catalog.LookupPricingEntry(context.Background(), "remote-model")
	require.NotNil(t, before)
	require.InDelta(t, 1e-6, *before.InputPrice, 1e-12)

	require.NoError(t, os.WriteFile(pricing.cfg.Pricing.FallbackFile, []byte(`{`+hotReloadModelJSON("remote-model", 9e-6, 1e-5)+`}`), 0644))
	pricing.runScheduledUpdate()

	after := catalog.LookupPricingEntry(context.Background(), "remote-model")
	require.NotNil(t, after)
	require.InDelta(t, 9e-6, *after.InputPrice, 1e-12, "seed entry must follow the reloaded pricing file")
}
