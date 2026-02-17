package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Versifine/locus/internal/event"
	"github.com/Versifine/locus/internal/llm"
	"github.com/Versifine/locus/internal/world"
)

func TestMemoryStoreRememberAutoTagsAndRecall(t *testing.T) {
	store := NewMemoryStore(10)
	ctx := MemoryContext{
		Player:    "Steve",
		Dimension: world.DimensionOverworld,
		Position:  [3]int{10, 64, 20},
		TickID:    100,
	}

	entry := store.Remember("在村庄北边看到铁匠铺", map[string]string{"type": "fact"}, ctx, "llm")
	if entry.ID == "" {
		t.Fatal("remember should create entry")
	}
	if entry.Tags["player"] != "Steve" {
		t.Fatalf("player tag=%q want Steve", entry.Tags["player"])
	}
	if entry.Tags["dim"] != world.DimensionOverworld {
		t.Fatalf("dim tag=%q want %q", entry.Tags["dim"], world.DimensionOverworld)
	}

	results := store.Recall("铁匠铺", nil, ctx, 5)
	if len(results) == 0 {
		t.Fatal("recall should return at least one result")
	}
	if results[0].Content != "在村庄北边看到铁匠铺" {
		t.Fatalf("content=%q", results[0].Content)
	}
	if results[0].Tick != 100 {
		t.Fatalf("tick=%d want 100", results[0].Tick)
	}
}

func TestMemoryStoreRecallSoftFilterAndExplicitOverride(t *testing.T) {
	store := NewMemoryStore(10)
	store.Remember("基地坐标是 [100,64,200]", map[string]string{"player": "Steve", "dim": world.DimensionOverworld}, MemoryContext{TickID: 10}, "llm")
	store.Remember("基地坐标是 [8,70,8]", map[string]string{"player": "Alex", "dim": world.DimensionNether}, MemoryContext{TickID: 20}, "llm")

	ctx := MemoryContext{Player: "Steve", Dimension: world.DimensionOverworld, TickID: 30}
	softResults := store.Recall("基地坐标", nil, ctx, 2)
	if len(softResults) < 2 {
		t.Fatalf("soft recall len=%d want >=2", len(softResults))
	}
	if softResults[0].Tags["player"] != "Steve" {
		t.Fatalf("soft filter first player=%q want Steve", softResults[0].Tags["player"])
	}

	overrideResults := store.Recall("基地坐标", map[string]string{"player": "Alex"}, ctx, 2)
	if len(overrideResults) == 0 {
		t.Fatal("explicit filter should return Alex result")
	}
	if overrideResults[0].Tags["player"] != "Alex" {
		t.Fatalf("override first player=%q want Alex", overrideResults[0].Tags["player"])
	}
}

func TestMemoryStoreCapacityEviction(t *testing.T) {
	store := NewMemoryStore(2)
	ctx := MemoryContext{TickID: 1}
	entryA := store.Remember("alpha", map[string]string{"type": "note"}, ctx, "llm")
	ctx.TickID = 2
	_ = store.Remember("beta", map[string]string{"type": "note"}, ctx, "llm")

	ctx.TickID = 3
	_ = store.Recall("alpha", nil, ctx, 1)

	ctx.TickID = 100
	entryC := store.Remember("gamma", map[string]string{"type": "note"}, ctx, "llm")

	snapshot := store.Snapshot()
	if len(snapshot) != 2 {
		t.Fatalf("snapshot len=%d want 2", len(snapshot))
	}

	hasA := false
	hasC := false
	for _, entry := range snapshot {
		if entry.ID == entryA.ID {
			hasA = true
		}
		if entry.ID == entryC.ID {
			hasC = true
		}
	}
	if !hasA || !hasC {
		t.Fatalf("expected hot+new entries kept, hasA=%v hasC=%v", hasA, hasC)
	}
}

func TestEpisodeLogOpenCloseAndFormat(t *testing.T) {
	log := NewEpisodeLog(10)
	open := log.Open(
		10,
		"chat@tick=10 user=Steve",
		"先观察",
		"set_intent(go_to)",
		7,
		[]string{"go_to"},
		[]BufferedEvent{{Name: "chat", TickID: 10}},
	)
	if open.ID == "" {
		t.Fatal("open episode should have ID")
	}

	closed, ok := log.CloseByBehaviorEnd(7, "go_to", "completed", 20)
	if !ok {
		t.Fatal("close by behavior end should succeed")
	}
	if !closed.Episode.Closed {
		t.Fatal("episode should be closed")
	}
	if closed.Episode.BehaviorRunID != 7 {
		t.Fatalf("behavior run id=%d want 7", closed.Episode.BehaviorRunID)
	}
	if !strings.Contains(closed.Episode.Outcome, "run_id=7") {
		t.Fatalf("outcome=%q should contain run_id=7", closed.Episode.Outcome)
	}

	formatted := log.FormatRecent(3)
	if formatted == "" || formatted == "- none" {
		t.Fatalf("unexpected formatted short-term memory: %q", formatted)
	}
}

func TestEpisodeLogCloseByRunIDAvoidsActionCollision(t *testing.T) {
	log := NewEpisodeLog(10)
	episodeA := log.Open(10, "triggerA", "thoughtA", "set_intent(go_to)", 101, []string{"go_to"}, nil)
	episodeB := log.Open(11, "triggerB", "thoughtB", "set_intent(go_to)", 202, []string{"go_to"}, nil)

	if _, ok := log.CloseByBehaviorEnd(202, "go_to", "failed", 20); !ok {
		t.Fatal("close run_id=202 should succeed")
	}
	if _, ok := log.CloseByBehaviorEnd(101, "go_to", "completed", 21); !ok {
		t.Fatal("close run_id=101 should succeed")
	}

	gotA, ok := log.GetByID(episodeA.ID)
	if !ok {
		t.Fatal("episodeA should exist")
	}
	gotB, ok := log.GetByID(episodeB.ID)
	if !ok {
		t.Fatal("episodeB should exist")
	}

	if !strings.Contains(gotA.Outcome, "run_id=101") || !strings.Contains(gotA.Outcome, "reason=completed") {
		t.Fatalf("episodeA outcome mismatch: %q", gotA.Outcome)
	}
	if !strings.Contains(gotB.Outcome, "run_id=202") || !strings.Contains(gotB.Outcome, "reason=failed") {
		t.Fatalf("episodeB outcome mismatch: %q", gotB.Outcome)
	}
}

func TestLoopAgentBehaviorEndBeforeEpisodeOpen(t *testing.T) {
	a := &LoopAgent{
		episodeLog:         NewEpisodeLog(10),
		memoryStore:        NewMemoryStore(10),
		autoRuleLastTick:   map[string]uint64{},
		episodeByRunID:     map[uint64]string{},
		pendingBehaviorEnd: map[uint64]pendingBehaviorEnd{},
		thinkCtxRuns:       map[uint64]string{},
	}
	a.tickCounter.Store(30)

	evt := event.BehaviorEndEvent{Name: "go_to", RunID: 42, Reason: "completed"}
	a.closeEpisodeByBehaviorEnd(evt, 25)

	if _, ok := a.popPendingBehaviorEnd(42); !ok {
		t.Fatal("behavior_end should be buffered when episode is not created yet")
	}
	// reinsert pending event for open-after-end path
	a.pushPendingBehaviorEnd(evt, 25)

	a.setThinkContext(20, []BufferedEvent{{Name: event.EventChat, TickID: 20}})
	a.onBehaviorStartedFromIntent(Intent{Action: "go_to"}, 42)
	a.onThinkerFinished(20, []BufferedEvent{{Name: event.EventChat, TickID: 20}}, ThinkerTrace{
		Thoughts: []string{"先去目标点"},
		ToolCalls: []llm.ToolContentBlock{{
			Type:  "tool_use",
			Name:  "set_intent",
			Input: map[string]any{"action": "go_to"},
		}},
	}, nil)

	if _, ok := a.lookupEpisodeByRunID(42); ok {
		t.Fatal("run_id mapping should be removed after episode is closed")
	}
	if _, ok := a.popPendingBehaviorEnd(42); ok {
		t.Fatal("pending behavior_end should be consumed after episode opens")
	}

	recent := a.episodeLog.Recent(2)
	if len(recent) == 0 {
		t.Fatal("expected at least one episode")
	}
	last := recent[len(recent)-1]
	if !last.Closed {
		t.Fatal("episode should be closed by buffered behavior_end")
	}
	if last.BehaviorRunID != 42 {
		t.Fatalf("run_id=%d want 42", last.BehaviorRunID)
	}
	if !strings.Contains(last.Outcome, "reason=completed") {
		t.Fatalf("outcome=%q should contain completed reason", last.Outcome)
	}
	if !strings.Contains(last.Decision, "set_intent(go_to)") {
		t.Fatalf("decision=%q should include set_intent(go_to)", last.Decision)
	}
	if !strings.Contains(last.Thought, "先去目标点") {
		t.Fatalf("thought=%q should include thinker thought", last.Thought)
	}
}

func TestLoopAgentActionEpisodeGetsThoughtDecisionOnThinkerFinish(t *testing.T) {
	a := &LoopAgent{
		episodeLog:         NewEpisodeLog(10),
		memoryStore:        NewMemoryStore(10),
		autoRuleLastTick:   map[string]uint64{},
		episodeByRunID:     map[uint64]string{},
		pendingBehaviorEnd: map[uint64]pendingBehaviorEnd{},
		thinkCtxRuns:       map[uint64]string{},
	}
	a.tickCounter.Store(80)

	events := []BufferedEvent{{Name: event.EventChat, TickID: 79}}
	a.setThinkContext(79, events)
	a.onBehaviorStartedFromIntent(Intent{Action: "go_to"}, 501)
	a.onThinkerFinished(79, events, ThinkerTrace{
		Thoughts: []string{"先看地图再过去"},
		ToolCalls: []llm.ToolContentBlock{{
			Type:  "tool_use",
			Name:  "set_intent",
			Input: map[string]any{"action": "go_to"},
		}},
	}, nil)

	episodeID, ok := a.lookupEpisodeByRunID(501)
	if !ok {
		t.Fatal("run_id should be bound to episode after thinker finish")
	}
	episode, ok := a.episodeLog.GetByID(episodeID)
	if !ok {
		t.Fatal("episode should exist")
	}
	if episode.Closed {
		t.Fatal("episode should remain open before behavior.end")
	}
	if !strings.Contains(episode.Decision, "set_intent(go_to)") {
		t.Fatalf("decision=%q should include set_intent(go_to)", episode.Decision)
	}
	if !strings.Contains(episode.Thought, "先看地图再过去") {
		t.Fatalf("thought=%q should include thinker thought", episode.Thought)
	}
}

func TestLoopAgentActionEpisodeTimeoutFallback(t *testing.T) {
	a := &LoopAgent{
		episodeLog:         NewEpisodeLog(10),
		memoryStore:        NewMemoryStore(10),
		autoRuleLastTick:   map[string]uint64{},
		episodeByRunID:     map[uint64]string{},
		pendingBehaviorEnd: map[uint64]pendingBehaviorEnd{},
		thinkCtxRuns:       map[uint64]string{},
	}
	a.tickCounter.Store(2000)

	episode := a.episodeLog.Open(1000, "trigger", "thought", "set_intent(go_to)", 9001, []string{"go_to"}, nil)
	a.bindEpisodeRun(9001, episode.ID)

	a.episodeLog.mu.Lock()
	for i := range a.episodeLog.records {
		if a.episodeLog.records[i].episode.ID == episode.ID {
			a.episodeLog.records[i].episode.CreatedAt = time.Now().Add(-episodeOpenTimeout - time.Second)
		}
	}
	a.episodeLog.mu.Unlock()

	a.closeTimedOutEpisodes()

	if _, ok := a.lookupEpisodeByRunID(9001); ok {
		t.Fatal("run_id mapping should be cleared after timeout close")
	}

	updated, ok := a.episodeLog.GetByID(episode.ID)
	if !ok {
		t.Fatal("episode should exist")
	}
	if !updated.Closed {
		t.Fatal("episode should be closed by timeout fallback")
	}
	if !strings.Contains(updated.Outcome, "behavior_timeout") {
		t.Fatalf("outcome=%q should contain behavior_timeout", updated.Outcome)
	}
}

func TestLoopAgentPendingBehaviorEndCleanup(t *testing.T) {
	a := &LoopAgent{
		pendingBehaviorEnd: map[uint64]pendingBehaviorEnd{},
	}
	a.tickCounter.Store(20000)

	a.pushPendingBehaviorEnd(event.BehaviorEndEvent{Name: "go_to", RunID: 1, Reason: "completed"}, 1)
	a.pushPendingBehaviorEnd(event.BehaviorEndEvent{Name: "go_to", RunID: 2, Reason: "completed"}, 19999)
	a.cleanupPendingBehaviorEnds(a.tickCounter.Load())

	if _, ok := a.popPendingBehaviorEnd(1); ok {
		t.Fatal("stale pending behavior_end should be removed")
	}
	if _, ok := a.popPendingBehaviorEnd(2); !ok {
		t.Fatal("fresh pending behavior_end should remain")
	}
}

func TestLoopAgentRememberRecallHandlers(t *testing.T) {
	a := &LoopAgent{
		stateProvider:    loopTestState{},
		memoryStore:      NewMemoryStore(10),
		autoRuleLastTick: map[string]uint64{},
		activePlayer:     "Steve",
	}
	a.tickCounter.Store(55)

	rememberResult, err := a.rememberMemory(context.Background(), "这里有村庄", map[string]any{"type": "fact"})
	if err != nil {
		t.Fatalf("rememberMemory error: %v", err)
	}
	if rememberResult["status"] != "ok" {
		t.Fatalf("remember status=%v want ok", rememberResult["status"])
	}

	recallResult, err := a.recallMemory(context.Background(), "村庄", nil, 5)
	if err != nil {
		t.Fatalf("recallMemory error: %v", err)
	}
	if recallResult["status"] != "ok" {
		t.Fatalf("recall status=%v want ok", recallResult["status"])
	}
	items, ok := recallResult["items"].([]map[string]any)
	if !ok || len(items) == 0 {
		t.Fatalf("recall items malformed: %T %+v", recallResult["items"], recallResult["items"])
	}
	tags, ok := items[0]["tags"].(map[string]string)
	if !ok {
		t.Fatalf("tags type=%T want map[string]string", items[0]["tags"])
	}
	if tags["player"] != "Steve" {
		t.Fatalf("player tag=%q want Steve", tags["player"])
	}
}
