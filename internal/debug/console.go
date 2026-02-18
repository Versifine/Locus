package debug

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Versifine/locus/internal/body"
	"github.com/Versifine/locus/internal/physics"
	"github.com/Versifine/locus/internal/world"
	"golang.org/x/term"
)

const (
	defaultTickInterval = 50 * time.Millisecond
	defaultMovePulse    = 50 * time.Millisecond
	yawStep             = float32(5.0)
	pitchStep           = float32(5.0)
)

type ControlledBody interface {
	Tick(input body.InputState) error
	PhysicsState() physics.PhysicsState
	SetLocalPosition(pos world.Position)
}

type StateProvider interface {
	GetState() world.Snapshot
}

type BlockQuerier interface {
	GetBlockState(x, y, z int) (int32, bool)
}

type Console struct {
	body          ControlledBody
	stateProvider StateProvider
	blockQuerier  BlockQuerier
	tickInterval  time.Duration
	movePulse     time.Duration

	mu            sync.Mutex
	currentInput  body.InputState
	forwardUntil  time.Time
	backwardUntil time.Time
	leftUntil     time.Time
	rightUntil    time.Time
	attackUntil   time.Time
	useUntil      time.Time
	commandMode   bool
	commandBuf    []rune
	statusWidth   int
}

func NewConsole(body ControlledBody, stateProvider StateProvider, blockQuerier BlockQuerier) *Console {
	return &Console{
		body:          body,
		stateProvider: stateProvider,
		blockQuerier:  blockQuerier,
		tickInterval:  defaultTickInterval,
		movePulse:     defaultMovePulse,
	}
}

func (c *Console) Start(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("console is nil")
	}
	if c.body == nil {
		return fmt.Errorf("console body is nil")
	}
	if c.stateProvider == nil {
		return fmt.Errorf("console state provider is nil")
	}
	if c.blockQuerier == nil {
		return fmt.Errorf("console block querier is nil")
	}

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("set terminal raw mode: %w", err)
	}
	defer func() {
		_ = term.Restore(fd, oldState)
		fmt.Print("\r\n")
	}()

	fmt.Println("[debug] console started (W/A/S/D pulse, Space, arrows, X, F attack click, R use click, V attack hold, T use hold, :, [ sneaking, ] sprint)")
	c.renderStatusLine()

	go c.tickLoop(ctx)

	reader := bufio.NewReader(os.Stdin)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		b, err := reader.ReadByte()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read console input: %w", err)
		}
		c.handleKey(reader, b)
	}
}

func (c *Console) tickLoop(ctx context.Context) {
	ticker := time.NewTicker(c.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			input := c.getInput()
			if err := c.body.Tick(input); err != nil && !isConnectionNotReadyErr(err) {
				slog.Debug("debug body tick failed", "error", err)
			}
			c.renderStatusLine()
		}
	}
}

func (c *Console) handleKey(reader *bufio.Reader, b byte) {
	if c.isCommandMode() {
		c.handleCommandByte(b)
		return
	}

	switch b {
	case ':':
		c.enterCommandMode()
		return
	case 'w', 'W':
		c.pulseForward()
	case 's', 'S':
		c.pulseBackward()
	case 'a', 'A':
		c.pulseLeft()
	case 'd', 'D':
		c.pulseRight()
	case ' ':
		c.pulseJump()
	case '[':
		c.toggleSneak()
	case ']':
		c.toggleSprint()
	case 'f', 'F':
		c.pulseAttack()
	case 'r', 'R':
		c.pulseUse()
	case 'v', 'V':
		c.toggleAttackHold()
	case 't', 'T':
		c.toggleUseHold()
	case 'x', 'X':
		c.clearInput()
	case 27: // ESC + arrow sequence
		next, err := reader.ReadByte()
		if err != nil || next != '[' {
			return
		}
		arrow, err := reader.ReadByte()
		if err != nil {
			return
		}
		switch arrow {
		case 'D': // left
			c.adjustYaw(-yawStep)
		case 'C': // right
			c.adjustYaw(yawStep)
		case 'A': // up
			c.adjustPitch(-pitchStep)
		case 'B': // down
			c.adjustPitch(pitchStep)
		}
	}
	c.renderStatusLine()
}

func (c *Console) enterCommandMode() {
	c.mu.Lock()
	c.commandMode = true
	c.commandBuf = c.commandBuf[:0]
	c.mu.Unlock()
	fmt.Print("\r\n:")
}

func (c *Console) handleCommandByte(b byte) {
	switch b {
	case 13, 10: // Enter
		c.mu.Lock()
		cmd := strings.TrimSpace(string(c.commandBuf))
		c.commandMode = false
		c.commandBuf = c.commandBuf[:0]
		c.mu.Unlock()

		fmt.Print("\r\n")
		if cmd != "" {
			c.executeCommand(cmd)
		}
		c.renderStatusLine()
		return
	case 27: // ESC cancel command mode
		c.mu.Lock()
		c.commandMode = false
		c.commandBuf = c.commandBuf[:0]
		c.mu.Unlock()
		fmt.Print("\r\n[debug] command cancelled\r\n")
		c.renderStatusLine()
		return
	case 8, 127: // Backspace
		c.mu.Lock()
		if len(c.commandBuf) > 0 {
			c.commandBuf = c.commandBuf[:len(c.commandBuf)-1]
		}
		buf := string(c.commandBuf)
		c.mu.Unlock()
		fmt.Printf("\r:%s ", buf)
		fmt.Printf("\r:%s", buf)
		return
	default:
		if b < 32 || b > 126 {
			return
		}
		c.mu.Lock()
		c.commandBuf = append(c.commandBuf, rune(b))
		buf := string(c.commandBuf)
		c.mu.Unlock()
		fmt.Printf("\r:%s", buf)
	}
}

func (c *Console) executeCommand(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "help":
		c.printHelp()
	case "state":
		ps := c.body.PhysicsState()
		fmt.Printf("[debug] physics pos=(%.3f,%.3f,%.3f) vel=(%.3f,%.3f,%.3f) ground=%t\r\n",
			ps.Position.X, ps.Position.Y, ps.Position.Z,
			ps.Velocity.X, ps.Velocity.Y, ps.Velocity.Z,
			ps.OnGround,
		)
	case "snap":
		fmt.Printf("[debug] %s\r\n", c.stateProvider.GetState().String())
	case "tp":
		if len(parts) != 4 {
			fmt.Printf("[debug] usage: :tp <x> <y> <z>\r\n")
			return
		}
		x, err1 := strconv.ParseFloat(parts[1], 64)
		y, err2 := strconv.ParseFloat(parts[2], 64)
		z, err3 := strconv.ParseFloat(parts[3], 64)
		if err1 != nil || err2 != nil || err3 != nil {
			fmt.Printf("[debug] invalid tp args\r\n")
			return
		}
		snap := c.stateProvider.GetState()
		c.body.SetLocalPosition(world.Position{
			X:     x,
			Y:     y,
			Z:     z,
			Yaw:   snap.Position.Yaw,
			Pitch: snap.Position.Pitch,
		})
		fmt.Printf("[debug] local tp set to (%.3f, %.3f, %.3f)\r\n", x, y, z)
	case "block":
		if len(parts) != 4 {
			fmt.Printf("[debug] usage: :block <x> <y> <z>\r\n")
			return
		}
		x, err1 := strconv.Atoi(parts[1])
		y, err2 := strconv.Atoi(parts[2])
		z, err3 := strconv.Atoi(parts[3])
		if err1 != nil || err2 != nil || err3 != nil {
			fmt.Printf("[debug] invalid block args\r\n")
			return
		}
		stateID, ok := c.blockQuerier.GetBlockState(x, y, z)
		if !ok {
			fmt.Printf("[debug] block (%d,%d,%d): unloaded\r\n", x, y, z)
			return
		}
		fmt.Printf("[debug] block (%d,%d,%d): state_id=%d\r\n", x, y, z, stateID)
	case "look":
		c.handleLookCommand(parts)
	case "hotbar":
		c.handleHotbarCommand(parts)
	case "attack_target":
		c.handleAttackTargetCommand(parts)
	case "break":
		c.handleBreakCommand(parts)
	case "place":
		c.handlePlaceCommand(parts)
	case "interact_target":
		c.handleInteractTargetCommand(parts)
	case "use_mode":
		c.handleUseModeCommand(parts)
	case "hands_clear":
		c.clearHands()
		fmt.Printf("[debug] hands state cleared\r\n")
	default:
		fmt.Printf("[debug] unknown command: %s\r\n", parts[0])
	}
}

func (c *Console) handleLookCommand(parts []string) {
	if len(parts) == 2 {
		entityID64, err := strconv.ParseInt(parts[1], 10, 32)
		if err != nil {
			fmt.Printf("[debug] invalid entity id\r\n")
			return
		}
		entityID := int32(entityID64)
		snap := c.stateProvider.GetState()
		for _, e := range snap.Entities {
			if e.EntityID == entityID {
				c.lookAt(e.X, e.Y, e.Z)
				fmt.Printf("[debug] look at entity %d\r\n", entityID)
				return
			}
		}
		fmt.Printf("[debug] entity %d not found\r\n", entityID)
		return
	}

	if len(parts) == 4 {
		x, err1 := strconv.ParseFloat(parts[1], 64)
		y, err2 := strconv.ParseFloat(parts[2], 64)
		z, err3 := strconv.ParseFloat(parts[3], 64)
		if err1 != nil || err2 != nil || err3 != nil {
			fmt.Printf("[debug] invalid look args\r\n")
			return
		}
		c.lookAt(x, y, z)
		fmt.Printf("[debug] look at (%.3f, %.3f, %.3f)\r\n", x, y, z)
		return
	}

	fmt.Printf("[debug] usage: :look <entity_id> or :look <x> <y> <z>\r\n")
}

func (c *Console) handleHotbarCommand(parts []string) {
	if len(parts) != 2 {
		fmt.Printf("[debug] usage: :hotbar <0-8|off>\r\n")
		return
	}
	if parts[1] == "off" {
		c.mu.Lock()
		c.currentInput.HotbarSlot = nil
		c.mu.Unlock()
		fmt.Printf("[debug] hotbar switching disabled\r\n")
		return
	}

	slot64, err := strconv.ParseInt(parts[1], 10, 8)
	if err != nil || slot64 < 0 || slot64 > 8 {
		fmt.Printf("[debug] invalid hotbar slot, need 0..8\r\n")
		return
	}
	slot := int8(slot64)
	c.mu.Lock()
	c.currentInput.HotbarSlot = &slot
	c.mu.Unlock()
	fmt.Printf("[debug] hotbar slot=%d\r\n", slot)
}

func (c *Console) handleAttackTargetCommand(parts []string) {
	if len(parts) != 2 {
		fmt.Printf("[debug] usage: :attack_target <entity_id|off>\r\n")
		return
	}
	if parts[1] == "off" {
		c.mu.Lock()
		c.currentInput.AttackTarget = nil
		c.mu.Unlock()
		fmt.Printf("[debug] attack target cleared\r\n")
		return
	}

	id64, err := strconv.ParseInt(parts[1], 10, 32)
	if err != nil {
		fmt.Printf("[debug] invalid entity id\r\n")
		return
	}
	id := int32(id64)
	c.mu.Lock()
	c.currentInput.AttackTarget = &id
	c.mu.Unlock()
	fmt.Printf("[debug] attack target=%d\r\n", id)
}

func (c *Console) handleBreakCommand(parts []string) {
	if len(parts) == 2 && parts[1] == "off" {
		c.mu.Lock()
		c.currentInput.BreakTarget = nil
		c.mu.Unlock()
		fmt.Printf("[debug] break target cleared\r\n")
		return
	}
	if len(parts) != 4 {
		fmt.Printf("[debug] usage: :break <x> <y> <z|off>\r\n")
		return
	}
	x, y, z, ok := parseXYZ(parts[1], parts[2], parts[3])
	if !ok {
		fmt.Printf("[debug] invalid break args\r\n")
		return
	}
	target := &physics.BlockPos{X: x, Y: y, Z: z}
	c.mu.Lock()
	c.currentInput.BreakTarget = target
	c.mu.Unlock()
	fmt.Printf("[debug] break target=(%d,%d,%d)\r\n", x, y, z)
}

func (c *Console) handlePlaceCommand(parts []string) {
	if len(parts) == 2 && parts[1] == "off" {
		c.mu.Lock()
		c.currentInput.PlaceTarget = nil
		c.mu.Unlock()
		fmt.Printf("[debug] place target cleared\r\n")
		return
	}
	if len(parts) != 5 {
		fmt.Printf("[debug] usage: :place <x> <y> <z> <face(0-5)|off>\r\n")
		return
	}
	x, y, z, ok := parseXYZ(parts[1], parts[2], parts[3])
	if !ok {
		fmt.Printf("[debug] invalid place args\r\n")
		return
	}
	face64, err := strconv.ParseInt(parts[4], 10, 32)
	if err != nil || face64 < 0 || face64 > 5 {
		fmt.Printf("[debug] invalid face, need 0..5\r\n")
		return
	}
	place := &physics.PlaceAction{
		Pos:  physics.BlockPos{X: x, Y: y, Z: z},
		Face: int(face64),
	}
	c.mu.Lock()
	c.currentInput.PlaceTarget = place
	c.mu.Unlock()
	fmt.Printf("[debug] place target=(%d,%d,%d) face=%d\r\n", x, y, z, int(face64))
}

func (c *Console) handleInteractTargetCommand(parts []string) {
	if len(parts) != 2 {
		fmt.Printf("[debug] usage: :interact_target <entity_id|off>\r\n")
		return
	}
	if parts[1] == "off" {
		c.mu.Lock()
		c.currentInput.InteractTarget = nil
		c.mu.Unlock()
		fmt.Printf("[debug] interact target cleared\r\n")
		return
	}
	id64, err := strconv.ParseInt(parts[1], 10, 32)
	if err != nil {
		fmt.Printf("[debug] invalid entity id\r\n")
		return
	}
	id := int32(id64)
	c.mu.Lock()
	c.currentInput.InteractTarget = &id
	c.mu.Unlock()
	fmt.Printf("[debug] interact target=%d\r\n", id)
}

func (c *Console) handleUseModeCommand(parts []string) {
	if len(parts) != 2 {
		fmt.Printf("[debug] usage: :use_mode <item|place|interact|off>\r\n")
		return
	}

	mode := parts[1]
	c.mu.Lock()
	defer c.mu.Unlock()

	switch mode {
	case "off":
		c.currentInput.Use = false
	case "item":
		c.currentInput.Use = true
		c.currentInput.PlaceTarget = nil
		c.currentInput.InteractTarget = nil
	case "place":
		if c.currentInput.PlaceTarget == nil {
			fmt.Printf("[debug] place target is empty, set it first via :place\r\n")
			return
		}
		c.currentInput.Use = true
		c.currentInput.InteractTarget = nil
	case "interact":
		if c.currentInput.InteractTarget == nil {
			fmt.Printf("[debug] interact target is empty, set it first via :interact_target\r\n")
			return
		}
		c.currentInput.Use = true
		c.currentInput.PlaceTarget = nil
	default:
		fmt.Printf("[debug] unknown use_mode: %s\r\n", mode)
		return
	}

	fmt.Printf("[debug] use_mode=%s\r\n", mode)
}

func (c *Console) lookAt(x, y, z float64) {
	snap := c.stateProvider.GetState()
	self := snap.Position

	dx := x - self.X
	dy := y - self.Y
	dz := z - self.Z

	yaw := float32(math.Atan2(-dx, dz) * 180.0 / math.Pi)
	horizontal := math.Sqrt(dx*dx + dz*dz)
	pitch := float32(-math.Atan2(dy, horizontal) * 180.0 / math.Pi)

	c.mu.Lock()
	c.currentInput.Yaw = normalizeYaw(yaw)
	c.currentInput.Pitch = clampPitch(pitch)
	c.mu.Unlock()
}

func (c *Console) printHelp() {
	fmt.Print("[debug] keys:\r\n")
	fmt.Print("  W/S/A/D: pulse movement (~180ms)\r\n")
	fmt.Print("  Space: toggle jump\r\n")
	fmt.Print("  [: toggle sneak (Shift fallback)\r\n")
	fmt.Print("  ]: toggle sprint (Ctrl fallback)\r\n")
	fmt.Print("  F: attack click (pulse)\r\n")
	fmt.Print("  R: use click (pulse)\r\n")
	fmt.Print("  V: toggle attack hold\r\n")
	fmt.Print("  T: toggle use hold\r\n")
	fmt.Print("  Arrow Left/Right: yaw +/-5\r\n")
	fmt.Print("  Arrow Up/Down: pitch +/-5\r\n")
	fmt.Print("  X: clear all input\r\n")
	fmt.Print("  : enter command mode\r\n")
	fmt.Print("[debug] commands:\r\n")
	fmt.Print("  :look <entity_id>\r\n")
	fmt.Print("  :look <x> <y> <z>\r\n")
	fmt.Print("  :block <x> <y> <z>\r\n")
	fmt.Print("  :tp <x> <y> <z>\r\n")
	fmt.Print("  :hotbar <0-8|off>\r\n")
	fmt.Print("  :attack_target <entity_id|off>\r\n")
	fmt.Print("  :break <x> <y> <z|off>\r\n")
	fmt.Print("  :place <x> <y> <z> <face(0-5)|off>\r\n")
	fmt.Print("  :interact_target <entity_id|off>\r\n")
	fmt.Print("  :use_mode <item|place|interact|off>\r\n")
	fmt.Print("  :hands_clear\r\n")
	fmt.Print("  :state\r\n")
	fmt.Print("  :snap\r\n")
	fmt.Print("  :help\r\n")
}

func (c *Console) renderStatusLine() {
	c.mu.Lock()
	if c.commandMode {
		c.mu.Unlock()
		return
	}
	input := c.currentInput
	width := c.statusWidth
	c.mu.Unlock()

	snap := c.stateProvider.GetState()
	ps := c.body.PhysicsState()

	hands := fmt.Sprintf("atk:%s use:%s hb:%s", boolLabel(input.Attack), boolLabel(input.Use), hotbarLabel(input.HotbarSlot))
	targets := compactTargetsLabel(input)
	line := fmt.Sprintf(
		"[mv:%s/%s/%s/%s j:%s %s %s y:%.0f p:%.0f pos:%.1f,%.1f,%.1f g:%s]",
		boolLabel(input.Forward),
		boolLabel(input.Backward),
		boolLabel(input.Left),
		boolLabel(input.Right),
		boolLabel(input.Jump),
		hands,
		targets,
		input.Yaw,
		input.Pitch,
		snap.Position.X,
		snap.Position.Y,
		snap.Position.Z,
		boolLabel(ps.OnGround),
	)

	padding := ""
	if width > len(line) {
		padding = strings.Repeat(" ", width-len(line))
	}
	fmt.Printf("\r%s%s", line, padding)

	c.mu.Lock()
	if len(line) > c.statusWidth {
		c.statusWidth = len(line)
	}
	c.mu.Unlock()
}

func (c *Console) toggle(update func(*body.InputState)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	update(&c.currentInput)
}

func (c *Console) adjustYaw(delta float32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.currentInput.Yaw = normalizeYaw(c.currentInput.Yaw + delta)
}

func (c *Console) adjustPitch(delta float32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.currentInput.Pitch = clampPitch(c.currentInput.Pitch + delta)
}

func (c *Console) getInput() body.InputState {
	c.mu.Lock()
	now := time.Now()
	c.applyMovementPulseLocked(now)
	c.applyHandsPulseLocked(now)
	input := c.currentInput
	c.mu.Unlock()

	input = c.applyAutoAim(input)

	c.mu.Lock()
	c.currentInput.AttackTarget = input.AttackTarget
	c.currentInput.BreakTarget = input.BreakTarget
	c.currentInput.PlaceTarget = input.PlaceTarget
	c.currentInput.InteractTarget = input.InteractTarget
	c.mu.Unlock()

	return input
}

func (c *Console) isCommandMode() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.commandMode
}

func boolLabel(v bool) string {
	if v {
		return "on"
	}
	return "off"
}

func entityTargetLabel(v *int32) string {
	if v == nil {
		return "-"
	}
	return strconv.FormatInt(int64(*v), 10)
}

func hotbarLabel(v *int8) string {
	if v == nil {
		return "-"
	}
	return strconv.FormatInt(int64(*v), 10)
}

func compactTargetsLabel(input body.InputState) string {
	parts := make([]string, 0, 4)
	if input.AttackTarget != nil {
		parts = append(parts, "at:"+entityTargetLabel(input.AttackTarget))
	}
	if input.BreakTarget != nil {
		parts = append(parts, "br:"+breakTargetLabel(input.BreakTarget))
	}
	if input.PlaceTarget != nil {
		parts = append(parts, "pl:"+placeTargetLabel(input.PlaceTarget))
	}
	if input.InteractTarget != nil {
		parts = append(parts, "it:"+entityTargetLabel(input.InteractTarget))
	}
	if len(parts) == 0 {
		return "tg:-"
	}
	return strings.Join(parts, " ")
}

func breakTargetLabel(v *physics.BlockPos) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%d,%d,%d", v.X, v.Y, v.Z)
}

func placeTargetLabel(v *physics.PlaceAction) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%d,%d,%d@%d", v.Pos.X, v.Pos.Y, v.Pos.Z, v.Face)
}

type aimBlockHit struct {
	pos  physics.BlockPos
	face int
	dist float64
}

type aimEntityHit struct {
	id   int32
	dist float64
}

func (c *Console) applyAutoAim(input body.InputState) body.InputState {
	snap := c.stateProvider.GetState()

	blockHit, hasBlock := c.findAimBlock(snap)
	entityHit, hasEntity := c.findAimEntity(snap)

	if input.Attack {
		if hasEntity && (!hasBlock || entityHit.dist < blockHit.dist) {
			id := entityHit.id
			input.AttackTarget = &id
			input.BreakTarget = nil
		} else if hasBlock {
			bp := blockHit.pos
			input.BreakTarget = &bp
			input.AttackTarget = nil
		} else {
			input.AttackTarget = nil
			input.BreakTarget = nil
		}
	}

	if input.Use {
		if hasBlock && (!hasEntity || blockHit.dist <= entityHit.dist) {
			place := &physics.PlaceAction{Pos: blockHit.pos, Face: blockHit.face}
			input.PlaceTarget = place
			input.InteractTarget = nil
		} else if hasEntity {
			id := entityHit.id
			input.InteractTarget = &id
			input.PlaceTarget = nil
		} else {
			input.PlaceTarget = nil
			input.InteractTarget = nil
		}
	}

	return input
}

func (c *Console) findAimBlock(snap world.Snapshot) (aimBlockHit, bool) {
	const maxDist = 5.0
	const step = 0.1

	ox := snap.Position.X
	oy := snap.Position.Y + 1.62
	oz := snap.Position.Z
	dx, dy, dz := lookDir(snap.Position.Yaw, snap.Position.Pitch)

	prevX := int(math.Floor(ox))
	prevY := int(math.Floor(oy))
	prevZ := int(math.Floor(oz))

	for dist := step; dist <= maxDist; dist += step {
		x := ox + dx*dist
		y := oy + dy*dist
		z := oz + dz*dist

		bx := int(math.Floor(x))
		by := int(math.Floor(y))
		bz := int(math.Floor(z))

		if bx == prevX && by == prevY && bz == prevZ {
			continue
		}

		stateID, ok := c.blockQuerier.GetBlockState(bx, by, bz)
		if ok && stateID != 0 {
			return aimBlockHit{
				pos:  physics.BlockPos{X: bx, Y: by, Z: bz},
				face: enteredFace(prevX, prevY, prevZ, bx, by, bz),
				dist: dist,
			}, true
		}

		prevX, prevY, prevZ = bx, by, bz
	}

	return aimBlockHit{}, false
}

func (c *Console) findAimEntity(snap world.Snapshot) (aimEntityHit, bool) {
	const maxDist = 5.0
	const maxRadius = 0.8

	ox := snap.Position.X
	oy := snap.Position.Y + 1.62
	oz := snap.Position.Z
	dx, dy, dz := lookDir(snap.Position.Yaw, snap.Position.Pitch)

	best := aimEntityHit{}
	found := false

	for _, e := range snap.Entities {
		tx := e.X - ox
		ty := (e.Y + 0.9) - oy
		tz := e.Z - oz

		proj := tx*dx + ty*dy + tz*dz
		if proj <= 0 || proj > maxDist {
			continue
		}

		cx := ox + dx*proj
		cy := oy + dy*proj
		cz := oz + dz*proj

		distToRay := math.Sqrt((e.X-cx)*(e.X-cx) + (e.Y+0.9-cy)*(e.Y+0.9-cy) + (e.Z-cz)*(e.Z-cz))
		if distToRay > maxRadius {
			continue
		}

		if !found || proj < best.dist {
			best = aimEntityHit{id: e.EntityID, dist: proj}
			found = true
		}
	}

	return best, found
}

func lookDir(yaw, pitch float32) (float64, float64, float64) {
	yawRad := float64(yaw) * math.Pi / 180.0
	pitchRad := float64(pitch) * math.Pi / 180.0
	x := -math.Sin(yawRad) * math.Cos(pitchRad)
	y := -math.Sin(pitchRad)
	z := math.Cos(yawRad) * math.Cos(pitchRad)
	return x, y, z
}

func enteredFace(prevX, prevY, prevZ, x, y, z int) int {
	if x > prevX {
		return 4 // west
	}
	if x < prevX {
		return 5 // east
	}
	if y > prevY {
		return 0 // down
	}
	if y < prevY {
		return 1 // up
	}
	if z > prevZ {
		return 2 // north
	}
	if z < prevZ {
		return 3 // south
	}
	return 1
}

func normalizeYaw(yaw float32) float32 {
	for yaw <= -180 {
		yaw += 360
	}
	for yaw > 180 {
		yaw -= 360
	}
	return yaw
}

func clampPitch(pitch float32) float32 {
	if pitch < -90 {
		return -90
	}
	if pitch > 90 {
		return 90
	}
	return pitch
}

func isConnectionNotReadyErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "connection is not initialized")
}

func (c *Console) pulseForward() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	c.currentInput.Forward = true
	c.forwardUntil = now.Add(c.movePulse)
	c.currentInput.Backward = false
	c.backwardUntil = time.Time{}
}

func (c *Console) pulseBackward() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	c.currentInput.Backward = true
	c.backwardUntil = now.Add(c.movePulse)
	c.currentInput.Forward = false
	c.forwardUntil = time.Time{}
}

func (c *Console) pulseLeft() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	c.currentInput.Left = true
	c.leftUntil = now.Add(c.movePulse)
	c.currentInput.Right = false
	c.rightUntil = time.Time{}
}

func (c *Console) pulseRight() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	c.currentInput.Right = true
	c.rightUntil = now.Add(c.movePulse)
	c.currentInput.Left = false
	c.leftUntil = time.Time{}
}
func (c *Console) pulseJump() {
	//跳一次
	c.mu.Lock()
	c.currentInput.Jump = true
	c.mu.Unlock()

	//100ms后取消跳跃输入
	go func() {
		time.Sleep(100 * time.Millisecond)
		c.mu.Lock()
		c.currentInput.Jump = false
		c.mu.Unlock()
	}()
}

func (c *Console) applyMovementPulseLocked(now time.Time) {
	if !c.forwardUntil.IsZero() && !now.Before(c.forwardUntil) {
		c.currentInput.Forward = false
		c.forwardUntil = time.Time{}
	}
	if !c.backwardUntil.IsZero() && !now.Before(c.backwardUntil) {
		c.currentInput.Backward = false
		c.backwardUntil = time.Time{}
	}
	if !c.leftUntil.IsZero() && !now.Before(c.leftUntil) {
		c.currentInput.Left = false
		c.leftUntil = time.Time{}
	}
	if !c.rightUntil.IsZero() && !now.Before(c.rightUntil) {
		c.currentInput.Right = false
		c.rightUntil = time.Time{}
	}
}

func (c *Console) applyHandsPulseLocked(now time.Time) {
	if !c.attackUntil.IsZero() && !now.Before(c.attackUntil) {
		c.currentInput.Attack = false
		c.attackUntil = time.Time{}
	}
	if !c.useUntil.IsZero() && !now.Before(c.useUntil) {
		c.currentInput.Use = false
		c.useUntil = time.Time{}
	}
}

func (c *Console) toggleSneak() {
	c.mu.Lock()
	c.currentInput.Sneak = !c.currentInput.Sneak
	if c.currentInput.Sneak {
		c.currentInput.Sprint = false
	}
	enabled := c.currentInput.Sneak
	c.mu.Unlock()
	slog.Debug("debug sneak toggled", "enabled", enabled)
}

func (c *Console) toggleAttackHold() {
	c.mu.Lock()
	c.currentInput.Attack = !c.currentInput.Attack
	if !c.currentInput.Attack {
		c.attackUntil = time.Time{}
	}
	enabled := c.currentInput.Attack
	c.mu.Unlock()
	slog.Debug("debug attack hold toggled", "enabled", enabled)
}

func (c *Console) toggleUseHold() {
	c.mu.Lock()
	c.currentInput.Use = !c.currentInput.Use
	if !c.currentInput.Use {
		c.useUntil = time.Time{}
	}
	enabled := c.currentInput.Use
	c.mu.Unlock()
	slog.Debug("debug use hold toggled", "enabled", enabled)
}

func (c *Console) pulseAttack() {
	c.mu.Lock()
	c.currentInput.Attack = true
	c.attackUntil = time.Now().Add(120 * time.Millisecond)
	c.mu.Unlock()
}

func (c *Console) pulseUse() {
	c.mu.Lock()
	c.currentInput.Use = true
	c.useUntil = time.Now().Add(120 * time.Millisecond)
	c.mu.Unlock()
}

func (c *Console) clearHands() {
	c.mu.Lock()
	c.currentInput.Attack = false
	c.currentInput.Use = false
	c.attackUntil = time.Time{}
	c.useUntil = time.Time{}
	c.currentInput.AttackTarget = nil
	c.currentInput.BreakTarget = nil
	c.currentInput.PlaceTarget = nil
	c.currentInput.InteractTarget = nil
	c.currentInput.HotbarSlot = nil
	c.mu.Unlock()
}

func (c *Console) toggleSprint() {
	c.mu.Lock()
	c.currentInput.Sprint = !c.currentInput.Sprint
	if c.currentInput.Sprint {
		c.currentInput.Sneak = false
	}
	enabled := c.currentInput.Sprint
	c.mu.Unlock()
	slog.Debug("debug sprint toggled", "enabled", enabled)
}

func (c *Console) clearInput() {
	c.mu.Lock()
	c.currentInput = body.InputState{}
	c.forwardUntil = time.Time{}
	c.backwardUntil = time.Time{}
	c.leftUntil = time.Time{}
	c.rightUntil = time.Time{}
	c.attackUntil = time.Time{}
	c.useUntil = time.Time{}
	c.mu.Unlock()
}

func parseXYZ(xs, ys, zs string) (int, int, int, bool) {
	x, err1 := strconv.Atoi(xs)
	y, err2 := strconv.Atoi(ys)
	z, err3 := strconv.Atoi(zs)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, 0, false
	}
	return x, y, z, true
}
