// Package gemini 是 Gemini 原生端点的模型对象形状。网关的 /v1beta/models 按模型目录生成（2026-09-29），
// 不再转发上游、也没有内置兜底列表。
package gemini

type Model struct {
	Name                       string   `json:"name"`
	DisplayName                string   `json:"displayName,omitempty"`
	Description                string   `json:"description,omitempty"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods,omitempty"`
}

type ModelsListResponse struct {
	Models []Model `json:"models"`
}

// NewModel 按模型名生成 Gemini 模型对象：name 带 models/ 前缀，调用方式是网关支持的两种。
func NewModel(model string) Model {
	methods := []string{"generateContent", "streamGenerateContent"}
	if model == "" {
		return Model{Name: "models/unknown", SupportedGenerationMethods: methods}
	}
	if len(model) >= 7 && model[:7] == "models/" {
		return Model{Name: model, SupportedGenerationMethods: methods}
	}
	return Model{Name: "models/" + model, SupportedGenerationMethods: methods}
}
