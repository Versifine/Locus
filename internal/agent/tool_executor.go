package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Versifine/locus/internal/world"
)

type InventoryItem struct {
	Slot  int    `json:"slot"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type InventorySnapshot struct {
	Hotbar []InventoryItem `json:"hotbar"`
	Main   []InventoryItem `json:"main"`
}

type InventoryProvider interface {
	GetInventorySnapshot() (InventorySnapshot, bool)
}

type ToolExecutor struct {
	SnapshotFn func() world.Snapshot
	World      BlockAccess
	Camera     Camera
	TickIDFn   func() uint64

	SpatialMemory *SpatialMemory

	SpeakChan  chan<- string
	IntentChan chan<- Intent
	CancelAll  func()
	SetHead    func(yaw, pitch float32)

	WaitForIdle func(ctx context.Context, timeout time.Duration) (map[string]any, error)
	Recall      func(ctx context.Context, query string, filter map[string]any, topK int) (map[string]any, error)
	Remember    func(ctx context.Context, content string, tags map[string]any) (map[string]any, error)

	Inventory InventoryProvider
}

func ExecuteTool(name string, input map[string]any, snapshotFn func() world.Snapshot, worldAccess BlockAccess) (string, error) {
	executor := ToolExecutor{
		SnapshotFn: snapshotFn,
		World:      worldAccess,
		Camera:     DefaultCamera(),
	}
	return executor.ExecuteTool(context.Background(), name, input)
}

func (e ToolExecutor) ExecuteTool(ctx context.Context, name string, input map[string]any) (string, error) {
	name = strings.TrimSpace(name)
	if input == nil {
		input = map[string]any{}
	}

	switch name {
	case "look":
		return e.executeLook(input)
	case "look_at":
		return e.executeLookAt(input)
	case "query_block":
		return e.executeQueryBlock(input)
	case "query_nearby":
		return e.executeQueryNearby(input)
	case "check_inventory":
		return e.executeCheckInventory()
	case "speak":
		return e.executeSpeak(ctx, input)
	case "stop":
		return e.executeStop()
	case "go_to":
		return e.executeActionIntent(ctx, "go_to", input)
	case "follow":
		return e.executeActionIntent(ctx, "follow", input)
	case "attack":
		return e.executeActionIntent(ctx, "attack", input)
	case "mine":
		return e.executeActionIntent(ctx, "mine", input)
	case "place_block":
		return e.executeActionIntent(ctx, "place_block", input)
	case "use_item":
		return e.executeActionIntent(ctx, "use_item", input)
	case "switch_slot":
		return e.executeActionIntent(ctx, "switch_slot", input)
	case "set_intent":
		return e.executeSetIntent(ctx, input)
	case "wait_for_idle":
		return e.executeWaitForIdle(ctx, input)
	case "recall":
		return e.executeRecall(ctx, input)
	case "remember":
		return e.executeRemember(ctx, input)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func (e ToolExecutor) executeLook(input map[string]any) (string, error) {
	snap, err := e.snapshot()
	if err != nil {
		return "", err
	}
	if e.World == nil {
		return "", fmt.Errorf("block access unavailable")
	}

	direction := strings.ToLower(strings.TrimSpace(asString(input["direction"])))
	if direction == "" {
		direction = "forward"
	}

	yaw := float64(snap.Position.Yaw)
	pitch := float64(snap.Position.Pitch)
	switch direction {
	case "forward":
	case "left":
		yaw += 90
	case "right":
		yaw -= 90
	case "back":
		yaw += 180
	case "up":
		pitch -= 45
	case "down":
		pitch += 45
	default:
		return "", fmt.Errorf("invalid direction: %s", direction)
	}

	if e.SetHead != nil {
		e.SetHead(float32(yaw), float32(pitch))
	}

	eye := Vec3{X: snap.Position.X, Y: snap.Position.Y + 1.62, Z: snap.Position.Z}
	blocks := e.camera().VisibleSurfaceBlocks(eye, yaw, pitch, e.World)
	entities := visibleEntitiesInFOV(snap, eye, yaw, pitch, e.camera().FOV, e.camera().MaxDist)

	result := map[string]any{
		"direction": direction,
		"blocks":    FormatBlocks(blocks),
		"entities":  FormatEntities(entities, snap.PlayerList),
	}
	if e.SpatialMemory != nil {
		e.SpatialMemory.UpdateBlocks(blocks, e.currentTickID())
		e.SpatialMemory.UpdateEntities(entities, e.currentTickID())
		e.SpatialMemory.GC()
	}
	return toJSONString(result), nil
}

func (e ToolExecutor) executeLookAt(input map[string]any) (string, error) {
	if e.World == nil {
		return "", fmt.Errorf("block access unavailable")
	}
	x, ok := asInt(input["x"])
	if !ok {
		return "", fmt.Errorf("look_at missing x")
	}
	y, ok := asInt(input["y"])
	if !ok {
		return "", fmt.Errorf("look_at missing y")
	}
	z, ok := asInt(input["z"])
	if !ok {
		return "", fmt.Errorf("look_at missing z")
	}
	radius := 3
	if rawRadius, ok := asInt(input["radius"]); ok {
		if rawRadius > 0 {
			radius = rawRadius
		}
	}

	blocks := make([]BlockInfo, 0, (radius*2+1)*(radius*2+1)*(radius*2+1))
	for by := y - radius; by <= y+radius; by++ {
		for bx := x - radius; bx <= x+radius; bx++ {
			for bz := z - radius; bz <= z+radius; bz++ {
				stateID, ok := e.World.GetBlockState(bx, by, bz)
				if !ok || isAirState(e.World, stateID) {
					continue
				}
				name, _ := e.World.GetBlockNameByStateID(stateID)
				if strings.TrimSpace(name) == "" {
					name = fmt.Sprintf("state_%d", stateID)
				}
				blocks = append(blocks, BlockInfo{Type: name, Pos: [3]int{bx, by, bz}})
			}
		}
	}

	result := map[string]any{
		"center": fmt.Sprintf("[%d,%d,%d]", x, y, z),
		"radius": radius,
		"blocks": FormatBlocks(blocks),
	}
	return toJSONString(result), nil
}

func (e ToolExecutor) executeQueryBlock(input map[string]any) (string, error) {
	if e.World == nil {
		return "", fmt.Errorf("block access unavailable")
	}
	x, ok := asInt(input["x"])
	if !ok {
		return "", fmt.Errorf("query_block missing x")
	}
	y, ok := asInt(input["y"])
	if !ok {
		return "", fmt.Errorf("query_block missing y")
	}
	z, ok := asInt(input["z"])
	if !ok {
		return "", fmt.Errorf("query_block missing z")
	}

	stateID, found := e.World.GetBlockState(x, y, z)
	if !found {
		return toJSONString(map[string]any{
			"position": fmt.Sprintf("[%d,%d,%d]", x, y, z),
			"status":   "unloaded",
		}), nil
	}

	name, ok := e.World.GetBlockNameByStateID(stateID)
	if !ok {
		name = fmt.Sprintf("state_%d", stateID)
	}

	result := map[string]any{
		"position": fmt.Sprintf("[%d,%d,%d]", x, y, z),
		"state_id": stateID,
		"name":     name,
		"solid":    e.World.IsSolid(x, y, z),
	}
	return toJSONString(result), nil
}

func (e ToolExecutor) executeQueryNearby(input map[string]any) (string, error) {
	if e.SpatialMemory == nil {
		return toJSONString(map[string]any{"status": "unavailable", "reason": "spatial_memory_not_ready"}), nil
	}

	snap, err := e.snapshot()
	if err != nil {
		return "", err
	}

	radius := defaultSpatialQueryRadius
	if rawRadius, ok := asFloat64(input["radius"]); ok && rawRadius > 0 {
		radius = rawRadius
	}

	typeFilter := strings.ToLower(strings.TrimSpace(asString(input["type_filter"])))
	if typeFilter == "" {
		typeFilter = "all"
	}
	if typeFilter != "all" && typeFilter != "entity" && typeFilter != "block" {
		return "", fmt.Errorf("invalid type_filter: %s", typeFilter)
	}

	maxAgeSec := 30
	if rawMaxAgeSec, ok := asInt(input["max_age_sec"]); ok && rawMaxAgeSec > 0 {
		maxAgeSec = rawMaxAgeSec
	}

	center := Vec3{X: snap.Position.X, Y: snap.Position.Y, Z: snap.Position.Z}
	entities, blocks := e.SpatialMemory.QueryNearby(center, radius, time.Duration(maxAgeSec)*time.Second)
	summaryEntities := entities
	summaryBlocks := blocks
	if typeFilter == "entity" {
		summaryBlocks = nil
	}
	if typeFilter == "block" {
		summaryEntities = nil
	}

	now := time.Now()
	entityItems := make([]map[string]any, 0, len(entities))
	if typeFilter != "block" {
		for _, memory := range entities {
			ageSec := int(now.Sub(memory.LastSeen).Seconds())
			if ageSec < 0 {
				ageSec = 0
			}
			entityItems = append(entityItems, map[string]any{
				"entity_id":      memory.EntityID,
				"type":           memory.Type,
				"name":           memory.Name,
				"position":       [3]float64{memory.X, memory.Y, memory.Z},
				"in_fov":         memory.InFOV,
				"last_seen_sec":  ageSec,
				"last_seen_tick": memory.TickID,
			})
		}
	}

	blockItems := make([]map[string]any, 0, len(blocks))
	if typeFilter != "entity" {
		for _, memory := range blocks {
			ageSec := int(now.Sub(memory.LastSeen).Seconds())
			if ageSec < 0 {
				ageSec = 0
			}
			blockItems = append(blockItems, map[string]any{
				"name":           memory.Name,
				"position":       memory.Pos,
				"last_seen_sec":  ageSec,
				"last_seen_tick": memory.TickID,
			})
		}
	}

	result := map[string]any{
		"status":      "ok",
		"center":      [3]float64{center.X, center.Y, center.Z},
		"radius":      radius,
		"type_filter": typeFilter,
		"max_age_sec": maxAgeSec,
		"entities":    entityItems,
		"blocks":      blockItems,
		"summary":     spatialSummaryFromMemories(summaryEntities, summaryBlocks, time.Duration(maxAgeSec)*time.Second),
	}
	return toJSONString(result), nil
}

func (e ToolExecutor) executeCheckInventory() (string, error) {
	if e.Inventory == nil {
		slog.Warn("check_inventory unavailable", "reason", "inventory_not_ready")
		return toJSONString(map[string]any{
			"status": "unavailable",
			"reason": "inventory_not_ready",
		}), nil
	}

	inventory, ok := e.Inventory.GetInventorySnapshot()
	if !ok {
		slog.Warn("check_inventory unavailable", "reason", "inventory_not_ready")
		return toJSONString(map[string]any{
			"status": "unavailable",
			"reason": "inventory_not_ready",
		}), nil
	}

	return toJSONString(map[string]any{
		"status":  "ok",
		"hotbar":  inventory.Hotbar,
		"main":    inventory.Main,
		"summary": summarizeInventory(inventory),
	}), nil
}

func (e ToolExecutor) executeSpeak(ctx context.Context, input map[string]any) (string, error) {
	if e.SpeakChan == nil {
		return "", fmt.Errorf("speak channel unavailable")
	}
	message := strings.TrimSpace(asString(input["message"]))
	if message == "" {
		return "", fmt.Errorf("speak message is empty")
	}

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case e.SpeakChan <- message:
	}

	return toJSONString(map[string]any{"status": "ok"}), nil
}

func (e ToolExecutor) executeStop() (string, error) {
	if e.CancelAll != nil {
		e.CancelAll()
	}
	return toJSONString(map[string]any{"status": "ok"}), nil
}

func (e ToolExecutor) executeActionIntent(ctx context.Context, action string, input map[string]any) (string, error) {
	if e.IntentChan == nil {
		return "", fmt.Errorf("intent channel unavailable")
	}
	if input == nil {
		input = map[string]any{}
	}
	input["action"] = action
	intent, err := ParseIntent(input)
	if err != nil {
		return "", err
	}

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case e.IntentChan <- intent:
	}

	return toJSONString(map[string]any{"status": "ok", "action": intent.Action}), nil
}

func (e ToolExecutor) executeSetIntent(ctx context.Context, input map[string]any) (string, error) {
	if e.IntentChan == nil {
		return "", fmt.Errorf("intent channel unavailable")
	}
	intent, err := ParseIntent(input)
	if err != nil {
		return "", err
	}

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case e.IntentChan <- intent:
	}

	return toJSONString(map[string]any{"status": "ok", "action": intent.Action}), nil
}

func (e ToolExecutor) executeWaitForIdle(ctx context.Context, input map[string]any) (string, error) {
	timeoutMs := 10000
	if timeoutRaw, ok := asInt(input["timeout_ms"]); ok && timeoutRaw > 0 {
		timeoutMs = timeoutRaw
	}

	if e.WaitForIdle == nil {
		return toJSONString(map[string]any{"status": "timeout"}), nil
	}

	result, err := e.WaitForIdle(ctx, time.Duration(timeoutMs)*time.Millisecond)
	if err != nil {
		return "", err
	}
	if result == nil {
		return toJSONString(map[string]any{"status": "timeout"}), nil
	}
	return toJSONString(result), nil
}

func (e ToolExecutor) executeRecall(ctx context.Context, input map[string]any) (string, error) {
	query := strings.TrimSpace(asString(input["query"]))
	if query == "" {
		return "", fmt.Errorf("recall missing query")
	}

	filter := map[string]any{}
	if raw, ok := input["filter"].(map[string]any); ok {
		filter = raw
	}
	topK := 5
	if rawTopK, ok := asInt(input["topK"]); ok && rawTopK > 0 {
		topK = rawTopK
	}

	if e.Recall == nil {
		return toJSONString(map[string]any{"status": "unavailable", "reason": "recall_not_ready"}), nil
	}
	result, err := e.Recall(ctx, query, filter, topK)
	if err != nil {
		return "", err
	}
	if result == nil {
		result = map[string]any{"status": "ok", "items": []any{}}
	}
	return toJSONString(result), nil
}

func (e ToolExecutor) executeRemember(ctx context.Context, input map[string]any) (string, error) {
	content := strings.TrimSpace(asString(input["content"]))
	if content == "" {
		return "", fmt.Errorf("remember missing content")
	}
	tags := map[string]any{}
	if raw, ok := input["tags"].(map[string]any); ok {
		tags = raw
	}

	if e.Remember == nil {
		return toJSONString(map[string]any{"status": "unavailable", "reason": "remember_not_ready"}), nil
	}
	result, err := e.Remember(ctx, content, tags)
	if err != nil {
		return "", err
	}
	if result == nil {
		result = map[string]any{"status": "ok"}
	}
	return toJSONString(result), nil
}

func (e ToolExecutor) snapshot() (world.Snapshot, error) {
	if e.SnapshotFn == nil {
		return world.Snapshot{}, fmt.Errorf("snapshot function unavailable")
	}
	return e.SnapshotFn(), nil
}

func (e ToolExecutor) camera() Camera {
	if e.Camera.FOV <= 0 || e.Camera.Width <= 0 || e.Camera.Height <= 0 || e.Camera.MaxDist <= 0 {
		return DefaultCamera()
	}
	return e.Camera
}

func (e ToolExecutor) currentTickID() uint64 {
	if e.TickIDFn == nil {
		return 0
	}
	return e.TickIDFn()
}

func summarizeInventory(inv InventorySnapshot) string {
	parts := make([]string, 0, len(inv.Hotbar)+len(inv.Main))
	for _, item := range inv.Hotbar {
		parts = append(parts, fmt.Sprintf("hotbar[%d]=%s x%d", item.Slot, item.Name, item.Count))
	}
	for _, item := range inv.Main {
		parts = append(parts, fmt.Sprintf("main[%d]=%s x%d", item.Slot, item.Name, item.Count))
	}
	if len(parts) == 0 {
		return "empty"
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}

func visibleEntitiesInFOV(snap world.Snapshot, eye Vec3, yaw, pitch, fov, maxDist float64) []world.Entity {
	if len(snap.Entities) == 0 {
		return nil
	}
	viewDir := directionFromYawPitch(yaw, pitch)
	maxAngle := fov / 2
	maxDistSq := maxDist * maxDist
	out := make([]world.Entity, 0, len(snap.Entities))

	for _, entity := range snap.Entities {
		dx := entity.X - eye.X
		dy := entity.Y - eye.Y
		dz := entity.Z - eye.Z
		distSq := dx*dx + dy*dy + dz*dz
		if distSq > maxDistSq || distSq < 1e-6 {
			continue
		}
		dist := math.Sqrt(distSq)
		dot := (dx*viewDir.X + dy*viewDir.Y + dz*viewDir.Z) / dist
		if dot > 1 {
			dot = 1
		}
		if dot < -1 {
			dot = -1
		}
		angle := math.Acos(dot) * 180.0 / math.Pi
		if angle <= maxAngle {
			out = append(out, entity)
		}
	}
	return out
}

func toJSONString(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}
