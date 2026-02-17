package agent

import (
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	defaultMemoryCapacity   = 200
	defaultEpisodeCapacity  = 128
	defaultRecallTopK       = 5
	embeddingDimensions     = 64
	autoRuleLongCooldown    = 36000
	autoRuleMediumCooldown  = 1200
	autoRuleShortCooldown   = 400
	shortTermEpisodeMaxSize = 6
)

type MemoryContext struct {
	Player    string
	Dimension string
	Position  [3]int
	TickID    uint64
}

type MemoryEntry struct {
	ID          string
	Content     string
	Tags        map[string]string
	Pos         [3]int
	TickID      uint64
	Embedding   []float32
	HitCount    int
	LastHitTick uint64
	Source      string
}

type RecallResult struct {
	Content string
	Tags    map[string]string
	Tick    uint64
	Score   float64
}

type MemoryStore struct {
	mu       sync.Mutex
	entries  []MemoryEntry
	capacity int
	seq      uint64
}

func NewMemoryStore(capacity int) *MemoryStore {
	if capacity <= 0 {
		capacity = defaultMemoryCapacity
	}
	return &MemoryStore{capacity: capacity}
}

func (s *MemoryStore) Remember(content string, tags map[string]string, ctx MemoryContext, source string) MemoryEntry {
	if s == nil {
		return MemoryEntry{}
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return MemoryEntry{}
	}

	if source == "" {
		source = "llm"
	}

	normalizedTags := normalizeTags(tags)
	if normalizedTags["player"] == "" && ctx.Player != "" {
		normalizedTags["player"] = ctx.Player
	}
	if normalizedTags["dim"] == "" && ctx.Dimension != "" {
		normalizedTags["dim"] = ctx.Dimension
	}
	if normalizedTags["type"] == "" {
		normalizedTags["type"] = "note"
	}

	embeddingText := buildEmbeddingText(content, normalizedTags, ctx.Position)
	entry := MemoryEntry{}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	entry = MemoryEntry{
		ID:          fmt.Sprintf("mem-%d", s.seq),
		Content:     content,
		Tags:        copyStringMap(normalizedTags),
		Pos:         ctx.Position,
		TickID:      ctx.TickID,
		Embedding:   textEmbedding(embeddingText),
		HitCount:    0,
		LastHitTick: ctx.TickID,
		Source:      source,
	}

	s.entries = append(s.entries, entry)
	s.evictLocked(ctx.TickID)
	return entry
}

func (s *MemoryStore) Recall(query string, filter map[string]string, ctx MemoryContext, topK int) []RecallResult {
	if s == nil {
		return nil
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	if topK <= 0 {
		topK = defaultRecallTopK
	}

	queryLower := strings.ToLower(query)
	queryTokens := tokenizeText(query)
	queryEmbedding := textEmbedding(query)
	normalizedFilter := normalizeTags(filter)

	type recallCandidate struct {
		index int
		score float64
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	candidates := make([]recallCandidate, 0, len(s.entries))
	for i := range s.entries {
		entry := s.entries[i]
		if !matchesExplicitFilter(entry, normalizedFilter) {
			continue
		}

		keyword := keywordScore(queryLower, queryTokens, entry)
		semantic := cosineSimilarity(queryEmbedding, entry.Embedding)
		if keyword <= 0 && semantic <= 0 {
			continue
		}

		score := keyword*2.0 + semantic
		score += softFilterBoost(entry, normalizedFilter, ctx)
		if score <= 0 {
			continue
		}
		candidates = append(candidates, recallCandidate{index: i, score: score})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if math.Abs(candidates[i].score-candidates[j].score) > 1e-6 {
			return candidates[i].score > candidates[j].score
		}
		left := s.entries[candidates[i].index]
		right := s.entries[candidates[j].index]
		if left.TickID != right.TickID {
			return left.TickID > right.TickID
		}
		return left.ID < right.ID
	})

	if len(candidates) > topK {
		candidates = candidates[:topK]
	}

	results := make([]RecallResult, 0, len(candidates))
	for _, candidate := range candidates {
		entry := &s.entries[candidate.index]
		entry.HitCount++
		if ctx.TickID > 0 {
			entry.LastHitTick = ctx.TickID
		}

		results = append(results, RecallResult{
			Content: entry.Content,
			Tags:    copyStringMap(entry.Tags),
			Tick:    entry.TickID,
			Score:   roundFloat(candidate.score, 4),
		})
	}

	return results
}

func (s *MemoryStore) Len() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

func (s *MemoryStore) Snapshot() []MemoryEntry {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]MemoryEntry, 0, len(s.entries))
	for _, entry := range s.entries {
		copied := entry
		copied.Tags = copyStringMap(entry.Tags)
		copied.Embedding = append([]float32(nil), entry.Embedding...)
		out = append(out, copied)
	}
	return out
}

func (s *MemoryStore) evictLocked(currentTick uint64) {
	for len(s.entries) > s.capacity {
		worstIdx := 0
		worstScore := coldScore(s.entries[0], currentTick)
		for i := 1; i < len(s.entries); i++ {
			score := coldScore(s.entries[i], currentTick)
			if score < worstScore {
				worstScore = score
				worstIdx = i
			}
		}
		s.entries = append(s.entries[:worstIdx], s.entries[worstIdx+1:]...)
	}
}

func coldScore(entry MemoryEntry, currentTick uint64) float64 {
	anchor := entry.LastHitTick
	if anchor == 0 {
		anchor = entry.TickID
	}
	var age float64
	if currentTick > anchor {
		age = float64(currentTick - anchor)
	}
	return float64(entry.HitCount)*1000 - age
}

type Episode struct {
	ID            string
	TickID        uint64
	Trigger       string
	Thought       string
	Decision      string
	Outcome       string
	BehaviorRunID uint64
	Closed        bool
	CreatedAt     time.Time
	ClosedAt      time.Time
}

type ClosedEpisode struct {
	Episode Episode
	Events  []BufferedEvent
	Actions []string
}

type episodeRecord struct {
	episode Episode
	events  []BufferedEvent
	actions []string
}

type EpisodeLog struct {
	mu       sync.Mutex
	records  []episodeRecord
	capacity int
	seq      uint64
}

func NewEpisodeLog(capacity int) *EpisodeLog {
	if capacity <= 0 {
		capacity = defaultEpisodeCapacity
	}
	return &EpisodeLog{capacity: capacity}
}

func (l *EpisodeLog) Open(tickID uint64, trigger, thought, decision string, runID uint64, actions []string, events []BufferedEvent) Episode {
	if l == nil {
		return Episode{}
	}

	trigger = strings.TrimSpace(trigger)
	thought = strings.TrimSpace(thought)
	decision = strings.TrimSpace(decision)

	l.mu.Lock()
	defer l.mu.Unlock()

	l.seq++
	record := episodeRecord{
		episode: Episode{
			ID:            fmt.Sprintf("ep-%d", l.seq),
			TickID:        tickID,
			Trigger:       trigger,
			Thought:       thought,
			Decision:      decision,
			BehaviorRunID: runID,
			Closed:        false,
			CreatedAt:     time.Now(),
		},
		events:  append([]BufferedEvent(nil), events...),
		actions: normalizeActions(actions),
	}

	l.records = append(l.records, record)
	l.trimLocked()
	return record.episode
}

func (l *EpisodeLog) CloseByBehaviorEnd(runID uint64, action, reason string, tickID uint64) (ClosedEpisode, bool) {
	if l == nil {
		return ClosedEpisode{}, false
	}
	if runID == 0 {
		return ClosedEpisode{}, false
	}
	action = strings.ToLower(strings.TrimSpace(action))
	reason = strings.TrimSpace(reason)

	l.mu.Lock()
	defer l.mu.Unlock()

	idx := l.findOpenByRunIDLocked(runID)
	if idx < 0 {
		return ClosedEpisode{}, false
	}

	outcome := fmt.Sprintf("behavior_end action=%s run_id=%d reason=%s", action, runID, reason)
	closed := l.closeLocked(idx, outcome, runID, tickID)
	return closed, true
}

func (l *EpisodeLog) CloseByID(episodeID, outcome string, tickID uint64) (ClosedEpisode, bool) {
	if l == nil {
		return ClosedEpisode{}, false
	}
	episodeID = strings.TrimSpace(episodeID)
	if episodeID == "" {
		return ClosedEpisode{}, false
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	for i := range l.records {
		rec := &l.records[i]
		if rec.episode.ID != episodeID || rec.episode.Closed {
			continue
		}
		closed := l.closeLocked(i, outcome, 0, tickID)
		return closed, true
	}
	return ClosedEpisode{}, false
}

func (l *EpisodeLog) CloseOldestOpen(outcome string, tickID uint64) (ClosedEpisode, bool) {
	if l == nil {
		return ClosedEpisode{}, false
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	idx := l.findOldestOpenLocked()
	if idx < 0 {
		return ClosedEpisode{}, false
	}
	closed := l.closeLocked(idx, outcome, 0, tickID)
	return closed, true
}

func (l *EpisodeLog) CloseExpiredOpen(maxAge time.Duration, now time.Time, tickID uint64) []ClosedEpisode {
	if l == nil || maxAge <= 0 {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	closed := make([]ClosedEpisode, 0)
	for i := range l.records {
		rec := l.records[i]
		if rec.episode.Closed {
			continue
		}
		if rec.episode.CreatedAt.IsZero() {
			continue
		}
		if now.Sub(rec.episode.CreatedAt) < maxAge {
			continue
		}

		action := "none"
		if len(rec.actions) > 0 {
			action = rec.actions[0]
		}
		outcome := fmt.Sprintf("behavior_timeout action=%s run_id=%d", action, rec.episode.BehaviorRunID)
		closed = append(closed, l.closeLocked(i, outcome, rec.episode.BehaviorRunID, tickID))
	}
	return closed
}

func (l *EpisodeLog) Recent(limit int) []Episode {
	if l == nil {
		return nil
	}
	if limit <= 0 {
		limit = shortTermEpisodeMaxSize
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.records) == 0 {
		return nil
	}
	start := 0
	if len(l.records) > limit {
		start = len(l.records) - limit
	}

	out := make([]Episode, 0, len(l.records)-start)
	for i := start; i < len(l.records); i++ {
		out = append(out, l.records[i].episode)
	}
	return out
}

func (l *EpisodeLog) FormatRecent(limit int) string {
	episodes := l.Recent(limit)
	if len(episodes) == 0 {
		return "- none"
	}

	lines := make([]string, 0, len(episodes))
	for _, episode := range episodes {
		state := "open"
		if episode.Closed {
			state = "closed"
		}
		line := fmt.Sprintf("- [%s] id=%s tick=%d trigger=%s decision=%s", state, episode.ID, episode.TickID, compactText(episode.Trigger, 80), compactText(episode.Decision, 80))
		if episode.Closed {
			line += " outcome=" + compactText(episode.Outcome, 80)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (l *EpisodeLog) findOpenByRunIDLocked(runID uint64) int {
	for i := range l.records {
		rec := l.records[i]
		if rec.episode.Closed {
			continue
		}
		if rec.episode.BehaviorRunID == runID {
			return i
		}
	}
	return -1
}

func (l *EpisodeLog) findOldestOpenLocked() int {
	for i := range l.records {
		if !l.records[i].episode.Closed {
			return i
		}
	}
	return -1
}

func (l *EpisodeLog) closeLocked(index int, outcome string, runID uint64, tickID uint64) ClosedEpisode {
	rec := &l.records[index]
	if rec.episode.Closed {
		return ClosedEpisode{
			Episode: rec.episode,
			Events:  append([]BufferedEvent(nil), rec.events...),
			Actions: append([]string(nil), rec.actions...),
		}
	}

	rec.episode.Closed = true
	rec.episode.Outcome = strings.TrimSpace(outcome)
	if runID > 0 {
		rec.episode.BehaviorRunID = runID
	}
	rec.episode.ClosedAt = time.Now()
	if rec.episode.TickID == 0 {
		rec.episode.TickID = tickID
	}

	return ClosedEpisode{
		Episode: rec.episode,
		Events:  append([]BufferedEvent(nil), rec.events...),
		Actions: append([]string(nil), rec.actions...),
	}
}

func (l *EpisodeLog) GetByID(episodeID string) (Episode, bool) {
	if l == nil {
		return Episode{}, false
	}
	episodeID = strings.TrimSpace(episodeID)
	if episodeID == "" {
		return Episode{}, false
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	for i := range l.records {
		if l.records[i].episode.ID != episodeID {
			continue
		}
		return l.records[i].episode, true
	}
	return Episode{}, false
}

func (l *EpisodeLog) trimLocked() {
	for len(l.records) > l.capacity {
		idx := -1
		for i := range l.records {
			if l.records[i].episode.Closed {
				idx = i
				break
			}
		}
		if idx < 0 {
			idx = 0
		}
		l.records = append(l.records[:idx], l.records[idx+1:]...)
	}
}

func normalizeTags(tags map[string]string) map[string]string {
	if len(tags) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(tags))
	for key, value := range tags {
		normalizedKey := normalizeTagKey(key)
		if normalizedKey == "" {
			continue
		}
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out[normalizedKey] = trimmed
	}
	return out
}

func normalizeTagKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	switch key {
	case "dimension":
		return "dim"
	default:
		return key
	}
}

func copyStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func buildEmbeddingText(content string, tags map[string]string, pos [3]int) string {
	parts := make([]string, 0, len(tags)+2)
	parts = append(parts, content)
	for key, value := range tags {
		parts = append(parts, key+":"+value)
	}
	parts = append(parts, fmt.Sprintf("pos:%d,%d,%d", pos[0], pos[1], pos[2]))
	return strings.Join(parts, " ")
}

func matchesExplicitFilter(entry MemoryEntry, filter map[string]string) bool {
	if len(filter) == 0 {
		return true
	}
	for key, value := range filter {
		entryValue, ok := entry.Tags[key]
		if !ok {
			return false
		}
		if !strings.EqualFold(strings.TrimSpace(entryValue), strings.TrimSpace(value)) {
			return false
		}
	}
	return true
}

func softFilterBoost(entry MemoryEntry, filter map[string]string, ctx MemoryContext) float64 {
	boost := 0.0
	if _, ok := filter["player"]; !ok && ctx.Player != "" {
		if strings.EqualFold(entry.Tags["player"], ctx.Player) {
			boost += 0.35
		} else {
			boost -= 0.05
		}
	}
	if _, ok := filter["dim"]; !ok && ctx.Dimension != "" {
		if strings.EqualFold(entry.Tags["dim"], ctx.Dimension) {
			boost += 0.20
		} else {
			boost -= 0.05
		}
	}
	return boost
}

func keywordScore(query string, tokens []string, entry MemoryEntry) float64 {
	searchText := strings.ToLower(entry.Content)
	for key, value := range entry.Tags {
		searchText += " " + strings.ToLower(key) + ":" + strings.ToLower(value)
	}
	searchText += fmt.Sprintf(" %d %d %d", entry.Pos[0], entry.Pos[1], entry.Pos[2])

	score := 0.0
	if query != "" && strings.Contains(searchText, query) {
		score += 1.2
	}
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if strings.Contains(searchText, token) {
			score += 0.5
		}
	}
	return score
}

func textEmbedding(text string) []float32 {
	vec := make([]float32, embeddingDimensions)
	tokens := tokenizeText(text)
	if len(tokens) == 0 {
		return vec
	}

	for _, token := range tokens {
		hasher := fnv.New32a()
		_, _ = hasher.Write([]byte(token))
		index := int(hasher.Sum32() % uint32(embeddingDimensions))
		vec[index] += 1
	}

	norm := float32(0)
	for _, value := range vec {
		norm += value * value
	}
	if norm == 0 {
		return vec
	}
	norm = float32(math.Sqrt(float64(norm)))
	for i := range vec {
		vec[i] /= norm
	}
	return vec
}

func tokenizeText(text string) []string {
	lower := strings.ToLower(text)
	parts := strings.FieldsFunc(lower, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(parts) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	length := len(a)
	if len(b) < length {
		length = len(b)
	}

	dot := float64(0)
	normA := float64(0)
	normB := float64(0)
	for i := 0; i < length; i++ {
		av := float64(a[i])
		bv := float64(b[i])
		dot += av * bv
		normA += av * av
		normB += bv * bv
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func normalizeActions(actions []string) []string {
	if len(actions) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(actions))
	out := make([]string, 0, len(actions))
	for _, action := range actions {
		action = strings.ToLower(strings.TrimSpace(action))
		if action == "" {
			continue
		}
		if _, ok := seen[action]; ok {
			continue
		}
		seen[action] = struct{}{}
		out = append(out, action)
	}
	return out
}

func compactText(text string, maxLen int) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if text == "" {
		return "none"
	}
	if maxLen <= 0 {
		maxLen = 80
	}
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	return string(runes[:maxLen]) + "..."
}

func roundFloat(value float64, precision int) float64 {
	if precision <= 0 {
		return math.Round(value)
	}
	pow := math.Pow10(precision)
	return math.Round(value*pow) / pow
}
