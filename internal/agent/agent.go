package agent

import "github.com/Versifine/locus/internal/event"

type Agent struct {
	bus *event.Bus
}

func NewAgent(bus *event.Bus) *Agent {
	bus.Subscribe("chat", event.ChatEventHandler)
	return &Agent{bus: bus}
}
