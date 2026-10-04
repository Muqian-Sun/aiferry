package geminicli

// Model is one entry of the Google One model set below.
type Model struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	DisplayName string `json:"display_name"`
	CreatedAt   string `json:"created_at"`
}

// GoogleOneModels is the conservative model set exposed for legacy Google One
// OAuth accounts. Newer subscription models are served through Antigravity
// OAuth rather than the retired consumer Gemini CLI / Code Assist channel.
var GoogleOneModels = []Model{
	{ID: "gemini-2.5-flash", Type: "model", DisplayName: "Gemini 2.5 Flash", CreatedAt: ""},
	{ID: "gemini-2.5-pro", Type: "model", DisplayName: "Gemini 2.5 Pro", CreatedAt: ""},
	{ID: "gemini-2.0-flash", Type: "model", DisplayName: "Gemini 2.0 Flash", CreatedAt: ""},
}

// GoogleOneModelMapping returns a new whitelist map for each account so callers
// cannot mutate the package-level catalog.
func GoogleOneModelMapping() map[string]string {
	mapping := make(map[string]string, len(GoogleOneModels))
	for _, model := range GoogleOneModels {
		mapping[model.ID] = model.ID
	}
	return mapping
}

// DefaultTestModel is the default model to preselect in test flows.
const DefaultTestModel = "gemini-2.0-flash"
