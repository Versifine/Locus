package agent

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Versifine/locus/internal/event"
	"github.com/Versifine/locus/internal/llm"
	"github.com/Versifine/locus/internal/world"
)

type pendingBehaviorEnd struct {
	event  event.BehaviorEndEvent
	tickID uint64
}

func (a *LoopAgent) shortTermMemoryForPrompt() string {
	if a == nil || a.episodeLog == nil {
		return "- none"
	}
	return a.episodeLog.FormatRecent(shortTermEpisodeMaxSize)
}

func (a *LoopAgent) observeIncomingEvent(evt incomingEvent) {
	if a == nil {
		return
	}

	if evt.name == event.EventChat {
		a.captureActivePlayer(evt.payload)
	}
	if evt.name == event.EventBehaviorEnd {
		be, ok := asBehaviorEndEvent(evt.payload)
		if ok {
			a.closeEpisodeByBehaviorEnd(be, evt.tickID)
		}
	}
}

func (a *LoopAgent) onThinkerFinished(startTick uint64, events []BufferedEvent, trace ThinkerTrace, thinkErr error) {
	if a == nil || a.episodeLog == nil {
		return
	}
	runs := a.snapshotThinkRuns(startTick)
	if len(runs) > 0 {
		a.openActionEpisodesFromThinker(startTick, events, trace, runs)
		return
	}

	if thinkErr != nil {
		outcome := "thinker_error"
		switch {
		case errors.Is(thinkErr, context.Canceled):
			outcome = "thinker_interrupted"
		case errors.Is(thinkErr, context.DeadlineExceeded):
			outcome = "thinker_timeout"
		default:
			outcome = "thinker_error: " + compactText(thinkErr.Error(), 120)
		}
		a.openAndCloseMetaEpisode(startTick, events, trace, outcome)
		return
	}

	a.openAndCloseMetaEpisode(startTick, events, trace, "end_turn_no_behavior")
}

func (a *LoopAgent) openActionEpisodesFromThinker(startTick uint64, events []BufferedEvent, trace ThinkerTrace, runs map[uint64]string) {
	if a == nil || a.episodeLog == nil || len(runs) == 0 {
		return
	}

	trigger := summarizeEpisodeTrigger(events)
	thought := summarizeThoughts(trace.Thoughts)
	decision := summarizeToolDecisions(trace.ToolCalls)

	runIDs := make([]uint64, 0, len(runs))
	for runID := range runs {
		runIDs = append(runIDs, runID)
	}
	sort.Slice(runIDs, func(i, j int) bool { return runIDs[i] < runIDs[j] })

	for _, runID := range runIDs {
		action := runs[runID]
		episode := a.episodeLog.Open(
			startTick,
			trigger,
			thought,
			decision,
			runID,
			[]string{action},
			events,
		)
		a.bindEpisodeRun(runID, episode.ID)

		if pending, ok := a.popPendingBehaviorEnd(runID); ok {
			a.closeEpisodeByBehaviorEnd(pending.event, pending.tickID)
		}
	}
}

func (a *LoopAgent) openAndCloseMetaEpisode(startTick uint64, events []BufferedEvent, trace ThinkerTrace, outcome string) {
	if a == nil || a.episodeLog == nil {
		return
	}
	episode := a.episodeLog.Open(
		startTick,
		summarizeEpisodeTrigger(events),
		summarizeThoughts(trace.Thoughts),
		summarizeToolDecisions(trace.ToolCalls),
		0,
		extractBehaviorActions(trace.ToolCalls),
		events,
	)
	closed, ok := a.episodeLog.CloseByID(episode.ID, outcome, a.tickCounter.Load())
	if ok {
		a.applyAutoMemoryRules(closed)
	}
}

func (a *LoopAgent) setThinkContext(startTick uint64, events []BufferedEvent) {
	if a == nil {
		return
	}
	a.thinkCtxMu.Lock()
	a.thinkCtxStartTick = startTick
	a.thinkCtxTrigger = summarizeEpisodeTrigger(events)
	a.thinkCtxEvents = append(a.thinkCtxEvents[:0], events...)
	for runID := range a.thinkCtxRuns {
		delete(a.thinkCtxRuns, runID)
	}
	a.thinkCtxMu.Unlock()
}

func (a *LoopAgent) clearThinkContext(startTick uint64) {
	if a == nil {
		return
	}
	a.thinkCtxMu.Lock()
	if a.thinkCtxStartTick == startTick {
		a.thinkCtxStartTick = 0
		a.thinkCtxTrigger = ""
		a.thinkCtxEvents = a.thinkCtxEvents[:0]
		for runID := range a.thinkCtxRuns {
			delete(a.thinkCtxRuns, runID)
		}
	}
	a.thinkCtxMu.Unlock()
}

func (a *LoopAgent) currentThinkContext() (uint64, string, []BufferedEvent, map[uint64]string) {
	if a == nil {
		return 0, "", nil, nil
	}
	a.thinkCtxMu.Lock()
	defer a.thinkCtxMu.Unlock()
	events := append([]BufferedEvent(nil), a.thinkCtxEvents...)
	runs := make(map[uint64]string, len(a.thinkCtxRuns))
	for runID, action := range a.thinkCtxRuns {
		runs[runID] = action
	}
	return a.thinkCtxStartTick, a.thinkCtxTrigger, events, runs
}

func (a *LoopAgent) onBehaviorStartedFromIntent(intent Intent, runID uint64) {
	if a == nil || a.episodeLog == nil || runID == 0 {
		return
	}

	startTick, trigger, events, _ := a.currentThinkContext()
	if startTick == 0 {
		startTick = a.tickCounter.Load()
		if strings.TrimSpace(trigger) == "" {
			trigger = "set_intent"
		}
		episode := a.episodeLog.Open(
			startTick,
			trigger,
			"none",
			"set_intent("+intent.Action+")",
			runID,
			[]string{intent.Action},
			events,
		)
		a.bindEpisodeRun(runID, episode.ID)
		if pending, ok := a.popPendingBehaviorEnd(runID); ok {
			a.closeEpisodeByBehaviorEnd(pending.event, pending.tickID)
		}
		return
	}

	a.registerThinkRun(runID, intent.Action)
}

func (a *LoopAgent) registerThinkRun(runID uint64, action string) {
	if a == nil || runID == 0 {
		return
	}
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		return
	}
	a.thinkCtxMu.Lock()
	if a.thinkCtxStartTick > 0 {
		a.thinkCtxRuns[runID] = action
	}
	a.thinkCtxMu.Unlock()
}

func (a *LoopAgent) snapshotThinkRuns(startTick uint64) map[uint64]string {
	if a == nil {
		return nil
	}
	a.thinkCtxMu.Lock()
	defer a.thinkCtxMu.Unlock()
	if a.thinkCtxStartTick != startTick {
		return nil
	}
	out := make(map[uint64]string, len(a.thinkCtxRuns))
	for runID, action := range a.thinkCtxRuns {
		out[runID] = action
	}
	return out
}

func (a *LoopAgent) closeEpisodeByBehaviorEnd(evt event.BehaviorEndEvent, tickID uint64) {
	if a == nil || a.episodeLog == nil {
		return
	}
	episodeID, ok := a.lookupEpisodeByRunID(evt.RunID)
	if !ok {
		a.pushPendingBehaviorEnd(evt, tickID)
		return
	}
	outcome := fmt.Sprintf("behavior_end action=%s run_id=%d reason=%s", strings.ToLower(strings.TrimSpace(evt.Name)), evt.RunID, strings.TrimSpace(evt.Reason))
	closed, ok := a.episodeLog.CloseByID(episodeID, outcome, tickID)
	if !ok {
		a.pushPendingBehaviorEnd(evt, tickID)
		return
	}
	a.unbindEpisodeRun(evt.RunID)
	a.applyAutoMemoryRules(closed)
}

func (a *LoopAgent) bindEpisodeRun(runID uint64, episodeID string) {
	if a == nil || runID == 0 || strings.TrimSpace(episodeID) == "" {
		return
	}
	a.episodeRunMu.Lock()
	a.episodeByRunID[runID] = episodeID
	a.episodeRunMu.Unlock()
}

func (a *LoopAgent) lookupEpisodeByRunID(runID uint64) (string, bool) {
	if a == nil || runID == 0 {
		return "", false
	}
	a.episodeRunMu.Lock()
	defer a.episodeRunMu.Unlock()
	episodeID, ok := a.episodeByRunID[runID]
	return episodeID, ok
}

func (a *LoopAgent) unbindEpisodeRun(runID uint64) {
	if a == nil || runID == 0 {
		return
	}
	a.episodeRunMu.Lock()
	delete(a.episodeByRunID, runID)
	a.episodeRunMu.Unlock()
}

func (a *LoopAgent) pushPendingBehaviorEnd(evt event.BehaviorEndEvent, tickID uint64) {
	if a == nil || evt.RunID == 0 {
		return
	}
	a.episodeRunMu.Lock()
	a.pendingBehaviorEnd[evt.RunID] = pendingBehaviorEnd{event: evt, tickID: tickID}
	a.cleanupPendingBehaviorEndsLocked(a.tickCounter.Load())
	a.episodeRunMu.Unlock()
}

func (a *LoopAgent) popPendingBehaviorEnd(runID uint64) (pendingBehaviorEnd, bool) {
	if a == nil || runID == 0 {
		return pendingBehaviorEnd{}, false
	}
	a.episodeRunMu.Lock()
	defer a.episodeRunMu.Unlock()
	value, ok := a.pendingBehaviorEnd[runID]
	if ok {
		delete(a.pendingBehaviorEnd, runID)
	}
	return value, ok
}

func (a *LoopAgent) dropPendingBehaviorEnd(runID uint64) {
	if a == nil || runID == 0 {
		return
	}
	a.episodeRunMu.Lock()
	delete(a.pendingBehaviorEnd, runID)
	a.episodeRunMu.Unlock()
}

func (a *LoopAgent) closeTimedOutEpisodes() {
	if a == nil || a.episodeLog == nil {
		return
	}
	closed := a.episodeLog.CloseExpiredOpen(episodeOpenTimeout, time.Now(), a.tickCounter.Load())
	for _, episode := range closed {
		if episode.Episode.BehaviorRunID > 0 {
			a.unbindEpisodeRun(episode.Episode.BehaviorRunID)
			a.dropPendingBehaviorEnd(episode.Episode.BehaviorRunID)
		}
		a.applyAutoMemoryRules(episode)
	}
}

func (a *LoopAgent) cleanupPendingBehaviorEnds(currentTick uint64) {
	if a == nil {
		return
	}
	a.episodeRunMu.Lock()
	a.cleanupPendingBehaviorEndsLocked(currentTick)
	a.episodeRunMu.Unlock()
}

func (a *LoopAgent) cleanupPendingBehaviorEndsLocked(currentTick uint64) {
	if len(a.pendingBehaviorEnd) == 0 {
		return
	}
	for runID, pending := range a.pendingBehaviorEnd {
		if pending.tickID == 0 {
			continue
		}
		if currentTick <= pending.tickID {
			continue
		}
		if currentTick-pending.tickID > pendingEndTTLInTicks {
			delete(a.pendingBehaviorEnd, runID)
		}
	}

	for len(a.pendingBehaviorEnd) > pendingEndMaxEntries {
		oldestRunID := uint64(0)
		oldestTick := uint64(0)
		for runID, pending := range a.pendingBehaviorEnd {
			if oldestRunID == 0 || pending.tickID < oldestTick {
				oldestRunID = runID
				oldestTick = pending.tickID
			}
		}
		if oldestRunID == 0 {
			break
		}
		delete(a.pendingBehaviorEnd, oldestRunID)
	}
}

func (a *LoopAgent) captureActivePlayer(payload any) {
	chat, ok := asChatEvent(payload)
	if !ok || chat == nil {
		return
	}
	if chat.Source != event.SourcePlayer {
		return
	}
	name := strings.TrimSpace(chat.Username)
	if name == "" {
		return
	}

	a.contextMu.Lock()
	a.activePlayer = name
	a.contextMu.Unlock()
}

func (a *LoopAgent) recallMemory(ctx context.Context, query string, filter map[string]any, topK int) (map[string]any, error) {
	if a == nil || a.memoryStore == nil {
		return map[string]any{"status": "unavailable", "reason": "memory_not_ready"}, nil
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("recall missing query")
	}

	snap := world.Snapshot{}
	if a.stateProvider != nil {
		snap = a.stateProvider.GetState()
	}
	memoryCtx := a.memoryContextFromSnapshot(snap, a.tickCounter.Load())
	items := a.memoryStore.Recall(query, anyMapToStringMap(filter), memoryCtx, topK)

	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"content": item.Content,
			"tags":    item.Tags,
			"tick":    item.Tick,
			"score":   item.Score,
		})
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	return map[string]any{
		"status": "ok",
		"items":  out,
	}, nil
}

func (a *LoopAgent) rememberMemory(ctx context.Context, content string, tags map[string]any) (map[string]any, error) {
	if a == nil || a.memoryStore == nil {
		return map[string]any{"status": "unavailable", "reason": "memory_not_ready"}, nil
	}

	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("remember missing content")
	}

	snap := world.Snapshot{}
	if a.stateProvider != nil {
		snap = a.stateProvider.GetState()
	}
	memoryCtx := a.memoryContextFromSnapshot(snap, a.tickCounter.Load())
	entry := a.memoryStore.Remember(content, anyMapToStringMap(tags), memoryCtx, "llm")

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if entry.ID == "" {
		return map[string]any{"status": "ignored"}, nil
	}

	return map[string]any{
		"status": "ok",
		"id":     entry.ID,
		"tick":   entry.TickID,
		"tags":   entry.Tags,
	}, nil
}

func (a *LoopAgent) memoryContextFromSnapshot(snap world.Snapshot, tickID uint64) MemoryContext {
	player := ""
	a.contextMu.Lock()
	player = a.activePlayer
	a.contextMu.Unlock()

	return MemoryContext{
		Player:    player,
		Dimension: snap.DimensionName,
		Position: [3]int{
			int(math.Round(snap.Position.X)),
			int(math.Round(snap.Position.Y)),
			int(math.Round(snap.Position.Z)),
		},
		TickID: tickID,
	}
}

func (a *LoopAgent) applyAutoMemoryRules(closed ClosedEpisode) {
	if a == nil || a.memoryStore == nil {
		return
	}

	snap := world.Snapshot{}
	if a.stateProvider != nil {
		snap = a.stateProvider.GetState()
	}
	tickID := a.tickCounter.Load()
	ctx := a.memoryContextFromSnapshot(snap, tickID)

	for _, evt := range closed.Events {
		if evt.Name != event.EventEntityAppear {
			continue
		}
		entityEvt, ok := asEntityEvent(evt.Payload)
		if !ok {
			continue
		}
		entityName := strings.TrimSpace(entityEvt.Name)
		if entityName == "" {
			continue
		}
		key := fmt.Sprintf("first-encounter:%s:%s", strings.ToLower(entityName), ctx.Dimension)
		if !a.allowAutoRule(key, tickID, autoRuleLongCooldown) {
			continue
		}
		a.memoryStore.Remember(
			fmt.Sprintf("首次发现实体 %s", entityName),
			map[string]string{"type": "first_encounter", "entity": entityName},
			ctx,
			"auto",
		)
	}

	if episodeHasEvent(closed.Events, event.EventDamage) {
		key := fmt.Sprintf("danger:%s:%d:%d", ctx.Dimension, ctx.Position[0], ctx.Position[2])
		if a.allowAutoRule(key, tickID, autoRuleMediumCooldown) {
			a.memoryStore.Remember(
				fmt.Sprintf("在 %s 受到伤害，需要保持警惕", emptyAsUnknown(ctx.Dimension)),
				map[string]string{"type": "danger"},
				ctx,
				"auto",
			)
		}
	}

	action, reason := parseBehaviorOutcome(closed.Episode.Outcome)
	if reason == "completed" && action != "" {
		key := fmt.Sprintf("task-complete:%s", action)
		if a.allowAutoRule(key, tickID, autoRuleShortCooldown) {
			a.memoryStore.Remember(
				fmt.Sprintf("完成行为 %s", action),
				map[string]string{"type": "task_complete", "action": action},
				ctx,
				"auto",
			)
		}

		if action == "go_to" {
			key = fmt.Sprintf("location:%s:%d:%d:%d", ctx.Dimension, ctx.Position[0], ctx.Position[1], ctx.Position[2])
			if a.allowAutoRule(key, tickID, autoRuleMediumCooldown) {
				a.memoryStore.Remember(
					fmt.Sprintf("到达坐标 [%d,%d,%d] (%s)", ctx.Position[0], ctx.Position[1], ctx.Position[2], emptyAsUnknown(ctx.Dimension)),
					map[string]string{"type": "location_fact"},
					ctx,
					"auto",
				)
			}
		}
	}
}

func (a *LoopAgent) allowAutoRule(key string, tickID uint64, cooldown uint64) bool {
	if a == nil {
		return false
	}
	key = strings.TrimSpace(strings.ToLower(key))
	if key == "" {
		return false
	}

	a.autoRuleMu.Lock()
	defer a.autoRuleMu.Unlock()

	lastTick := a.autoRuleLastTick[key]
	if tickID > 0 && lastTick > 0 && tickID-lastTick < cooldown {
		return false
	}
	a.autoRuleLastTick[key] = tickID
	return true
}

func summarizeEpisodeTrigger(events []BufferedEvent) string {
	if len(events) == 0 {
		return "none"
	}
	limit := len(events)
	if limit > 4 {
		limit = 4
	}
	lines := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		lines = append(lines, compactText(formatBufferedEvent(events[i]), 90))
	}
	if len(events) > limit {
		lines = append(lines, fmt.Sprintf("+%d more", len(events)-limit))
	}
	return strings.Join(lines, " | ")
}

func summarizeThoughts(thoughts []string) string {
	if len(thoughts) == 0 {
		return "none"
	}
	limit := len(thoughts)
	if limit > 3 {
		limit = 3
	}
	out := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, compactText(thoughts[i], 90))
	}
	return strings.Join(out, " | ")
}

func summarizeToolDecisions(calls []llm.ToolContentBlock) string {
	if len(calls) == 0 {
		return "none"
	}
	limit := len(calls)
	if limit > 6 {
		limit = 6
	}
	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		call := calls[i]
		if call.Name == "set_intent" {
			action := strings.TrimSpace(fmt.Sprintf("%v", call.Input["action"]))
			if action == "" {
				action = "unknown"
			}
			parts = append(parts, "set_intent("+action+")")
			continue
		}
		if call.Name == "speak" {
			msg := strings.TrimSpace(fmt.Sprintf("%v", call.Input["message"]))
			parts = append(parts, "speak("+compactText(msg, 30)+")")
			continue
		}
		parts = append(parts, call.Name)
	}
	if len(calls) > limit {
		parts = append(parts, fmt.Sprintf("+%d more", len(calls)-limit))
	}
	return strings.Join(parts, ", ")
}

func extractBehaviorActions(calls []llm.ToolContentBlock) []string {
	actions := make([]string, 0, len(calls))
	seen := map[string]struct{}{}
	for _, call := range calls {
		name := strings.ToLower(strings.TrimSpace(call.Name))
		action := ""
		switch name {
		case "go_to", "follow", "attack", "mine", "place_block", "use_item", "switch_slot", "idle", "look_at":
			action = name
		case "set_intent":
			if raw, ok := call.Input["action"]; ok {
				action = strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", raw)))
			}
		}
		if action == "" {
			continue
		}
		if _, exists := seen[action]; exists {
			continue
		}
		seen[action] = struct{}{}
		actions = append(actions, action)
	}
	return actions
}

func anyMapToStringMap(src map[string]any) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(src))
	for key, value := range src {
		text := strings.TrimSpace(fmt.Sprintf("%v", value))
		if text == "" || text == "<nil>" {
			continue
		}
		out[normalizeTagKey(key)] = text
	}
	return out
}

func episodeHasEvent(events []BufferedEvent, name string) bool {
	for _, evt := range events {
		if evt.Name == name {
			return true
		}
	}
	return false
}

func parseBehaviorOutcome(outcome string) (action, reason string) {
	parts := strings.Fields(strings.TrimSpace(outcome))
	for _, part := range parts {
		if strings.HasPrefix(part, "action=") {
			action = strings.TrimSpace(strings.TrimPrefix(part, "action="))
		}
		if strings.HasPrefix(part, "reason=") {
			reason = strings.TrimSpace(strings.TrimPrefix(part, "reason="))
		}
	}
	return strings.ToLower(action), strings.ToLower(reason)
}

func emptyAsUnknown(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "unknown"
	}
	return trimmed
}
