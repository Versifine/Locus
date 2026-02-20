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
		Name:        "query_nearby",
		Description: "查询空间记忆中附近的实体和方块（无需重新观察）",
		Parameters: map[string]ParamDef{
			"radius":      {Type: "integer", Default: 16},
			"type_filter": {Type: "string", Enum: []string{"entity", "block", "all"}, Default: "all"},
			"max_age_sec": {Type: "integer", Default: 30},
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
		Name:        "stop",
		Description: "立即停止所有行为，bot 原地站定",
		Parameters:  map[string]ParamDef{},
	},
	{
		Name:        "go_to",
		Description: "走到目标坐标（自动寻路）",
		Parameters: map[string]ParamDef{
			"x":      {Type: "integer", Required: true, Description: "目标 X 坐标"},
			"y":      {Type: "integer", Required: true, Description: "目标 Y 坐标"},
			"z":      {Type: "integer", Required: true, Description: "目标 Z 坐标"},
			"sprint": {Type: "boolean", Description: "是否疾跑"},
		},
	},
	{
		Name:        "follow",
		Description: "跟随指定实体移动",
		Parameters: map[string]ParamDef{
			"entity_id": {Type: "integer", Required: true, Description: "实体 ID"},
			"distance":  {Type: "number", Description: "保持距离（默认 3）"},
			"sprint":    {Type: "boolean", Description: "是否疾跑跟随"},
		},
	},
	{
		Name:        "attack",
		Description: "攻击指定实体",
		Parameters: map[string]ParamDef{
			"entity_id": {Type: "integer", Required: true, Description: "实体 ID"},
		},
	},
	{
		Name:        "mine",
		Description: "挖掘指定坐标的方块",
		Parameters: map[string]ParamDef{
			"x":    {Type: "integer", Required: true},
			"y":    {Type: "integer", Required: true},
			"z":    {Type: "integer", Required: true},
			"slot": {Type: "integer", Description: "快捷栏槽位 0-8"},
		},
	},
	{
		Name:        "place_block",
		Description: "在指定坐标放置方块",
		Parameters: map[string]ParamDef{
			"x":    {Type: "integer", Required: true},
			"y":    {Type: "integer", Required: true},
			"z":    {Type: "integer", Required: true},
			"face": {Type: "integer", Required: true, Description: "放置面 0=下 1=上 2=北 3=南 4=西 5=东"},
			"slot": {Type: "integer", Description: "快捷栏槽位 0-8"},
		},
	},
	{
		Name:        "use_item",
		Description: "使用当前手持物品",
		Parameters: map[string]ParamDef{
			"slot": {Type: "integer", Description: "快捷栏槽位 0-8"},
		},
	},
	{
		Name:        "switch_slot",
		Description: "切换快捷栏选中槽位",
		Parameters: map[string]ParamDef{
			"slot": {Type: "integer", Required: true, Description: "槽位 0-8"},
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
