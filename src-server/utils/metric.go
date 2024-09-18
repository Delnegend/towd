package utils

type Metric struct {
	DatabaseRead       chan float64
	DatabaseWrite      chan float64
	DiscordSendMessage chan float64
}

func NewMetric() *Metric {
	return &Metric{
		DatabaseRead:       make(chan float64),
		DatabaseWrite:      make(chan float64),
		DiscordSendMessage: make(chan float64),
	}
}
