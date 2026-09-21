package service

import (
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// 请求体 JSON 的两个小改写：账号级 model_mapping 改写上游模型名、续链失配时去掉 previous_response_id。

// ReplaceModelInBody 替换请求体 JSON 中的 model 字段。
func ReplaceModelInBody(body []byte, newModel string) []byte {
	if len(body) == 0 {
		return body
	}
	if current := gjson.GetBytes(body, "model"); current.Exists() && current.String() == newModel {
		return body
	}
	newBody, err := sjson.SetBytes(body, "model", newModel)
	if err != nil {
		return body
	}
	return newBody
}

// RemovePreviousResponseIDFromBody 删除请求体中的 previous_response_id，用于会话失配时改用完整 input 重建上下文。
func RemovePreviousResponseIDFromBody(body []byte) []byte {
	if len(body) == 0 {
		return body
	}
	if !gjson.GetBytes(body, "previous_response_id").Exists() {
		return body
	}
	newBody, err := sjson.DeleteBytes(body, "previous_response_id")
	if err != nil {
		return body
	}
	return newBody
}
