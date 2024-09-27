package utils

type Metric struct {
	DatabaseReadForAuthMiddleware chan float64 // required
	DatabaseReadForKanbanBoard    chan float64 // required
	DatabaseWriteForKanbanBoard   chan float64 // required

	DatabaseRead       chan float64 // required
	DatabaseWrite      chan float64 // required
	DiscordSendMessage chan float64 // required
}

func NewMetric() *Metric {
	return &Metric{
		DatabaseReadForAuthMiddleware: make(chan float64),
		DatabaseReadForKanbanBoard:    make(chan float64),
		DatabaseWriteForKanbanBoard:   make(chan float64),

		DatabaseRead:       make(chan float64),
		DatabaseWrite:      make(chan float64),
		DiscordSendMessage: make(chan float64),
	}
}
