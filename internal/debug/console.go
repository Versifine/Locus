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
	defaultMovePulse    = 180 * time.Millisecond
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

	fmt.Println("[debug] console started (W/A/S/D pulse, Space, arrows, X, :, [ sneaking, ] sprint)")
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
		c.toggle(func(in *body.InputState) { in.Jump = !in.Jump })
	case '[':
		c.toggleSneak()
	case ']':
		c.toggleSprint()
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
	fmt.Print("  Arrow Left/Right: yaw +/-5\r\n")
	fmt.Print("  Arrow Up/Down: pitch +/-5\r\n")
	fmt.Print("  X: clear all input\r\n")
	fmt.Print("  : enter command mode\r\n")
	fmt.Print("[debug] commands:\r\n")
	fmt.Print("  :look <entity_id>\r\n")
	fmt.Print("  :look <x> <y> <z>\r\n")
	fmt.Print("  :block <x> <y> <z>\r\n")
	fmt.Print("  :tp <x> <y> <z>\r\n")
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

	line := fmt.Sprintf(
		"[FWD:%s SPR:%s SNK:%s JMP:%s | YAW:%.1f PIT:%.1f | X:%.2f Y:%.2f Z:%.2f ground:%t]",
		boolLabel(input.Forward),
		boolLabel(input.Sprint),
		boolLabel(input.Sneak),
		boolLabel(input.Jump),
		input.Yaw,
		input.Pitch,
		snap.Position.X,
		snap.Position.Y,
		snap.Position.Z,
		ps.OnGround,
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
	c.applyMovementPulseLocked(time.Now())
	defer c.mu.Unlock()
	return c.currentInput
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
	c.mu.Unlock()
}
