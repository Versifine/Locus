package agent

import "github.com/Versifine/locus/internal/llm"

type ParamDef struct {
	Type        string              `json:"type,omitempty"`
	Description string              `json:"description,omitempty"`
	Enum        []string            `json:"enum,omitempty"`
	Required    bool                `json:"required,omitempty"`
	Default     any                 `json:"default,omitempty"`
	Properties  map[string]ParamDef `json:"properties,omitempty"`
}

type ToolDef struct {
	Name        string
	Description string
	Parameters  map[string]ParamDef
}

var PerceptionTools = []ToolDef{
	{
		Name:        "look",
		Description: "获取某方向视锥内的方块和实体",
		Parameters: map[string]ParamDef{
			"direction": {
				Type:        "string",
				Description: "观察方向",
				Enum:        []string{"forward", "left", "right", "back", "up", "down"},
				Required:    true,
			},
		},
	},
	{
		Name:        "look_at",
		Description: "看向特定坐标附近的详细方块",
		Parameters: map[string]ParamDef{
			"x":      {Type: "integer", Required: true},
			"y":      {Type: "integer", Required: true},
			"z":      {Type: "integer", Required: true},
			"radius": {Type: "integer", Default: 3},
		},
	},
	{
		Name:        "check_inventory",
		Description: "返回详细背包内容",
		Parameters:  map[string]ParamDef{},
	},
	{
		Name:        "query_block",
		Description: "查询特定坐标方块详情",
		Parameters: map[string]ParamDef{
			"x": {Type: "integer", Required: true},
			"y": {Type: "integer", Required: true},
			"z": {Type: "integer", Required: true},
		},
	},
	{
		Name:        "recall",
		Description: "混合检索长期记忆",
		Parameters: map[string]ParamDef{
			"query":  {Type: "string", Required: true},
			"filter": {Type: "object"},
			"topK":   {Type: "integer"},
		},
	},
	{
		Name:        "remember",
		Description: "写入长期记忆",
		Parameters: map[string]ParamDef{
			"content": {Type: "string", Required: true},
			"tags":    {Type: "object"},
		},
	},
}

var ActionTools = []ToolDef{
	{
		Name:        "speak",
		Description: "发送聊天消息",
		Parameters: map[string]ParamDef{
			"message": {Type: "string", Required: true},
		},
	},
	{
		Name:        "set_intent",
		Description: "设置行为意图，启动 Behavior",
		Parameters: map[string]ParamDef{
			"action": {Type: "string", Required: true},
		},
	},
	{
		Name:        "wait_for_idle",
		Description: "阻塞等待当前行为结束（不消耗 LLM token）",
		Parameters: map[string]ParamDef{
			"timeout_ms": {Type: "integer", Default: 10000},
		},
	},
}

func AllTools() []ToolDef {
	out := make([]ToolDef, 0, len(PerceptionTools)+len(ActionTools))
	out = append(out, PerceptionTools...)
	out = append(out, ActionTools...)
	return out
}

func ToLLMTools(defs []ToolDef) []llm.ToolDefinition {
	if len(defs) == 0 {
		return nil
	}
	out := make([]llm.ToolDefinition, 0, len(defs))
	for _, def := range defs {
		out = append(out, llm.ToolDefinition{
			Name:        def.Name,
			Description: def.Description,
			Parameters:  buildJSONSchema(def.Parameters),
		})
	}
	return out
}

func buildJSONSchema(params map[string]ParamDef) map[string]any {
	properties := make(map[string]any, len(params))
	required := make([]string, 0, len(params))
	for name, def := range params {
		properties[name] = paramDefToSchema(def)
		if def.Required {
			required = append(required, name)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func paramDefToSchema(def ParamDef) map[string]any {
	schema := map[string]any{}
	if def.Type != "" {
		schema["type"] = def.Type
	}
	if def.Description != "" {
		schema["description"] = def.Description
	}
	if len(def.Enum) > 0 {
		schema["enum"] = def.Enum
	}
	if def.Default != nil {
		schema["default"] = def.Default
	}
	if len(def.Properties) > 0 {
		props := make(map[string]any, len(def.Properties))
		for key, child := range def.Properties {
			props[key] = paramDefToSchema(child)
		}
		schema["properties"] = props
	}
	if len(schema) == 0 {
		schema["type"] = "string"
	}
	return schema
}
