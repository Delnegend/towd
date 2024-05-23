package ical

import (
	"bufio"
	"log/slog"
	"os"
	"strings"
	"time"
	"towd/src-server/utils"

	"github.com/google/uuid"
)

type Calendar struct {
	id          string
	name        string
	description string
	version     string
	timezone    string // use when the (datetime != UTC) & TZID not provided
	url         string
	events      []Event
}

func NewCalendar() Calendar {
	return Calendar{
		id: uuid.NewString(),
	}
}


// #region Getters

func (c *Calendar) GetId() string {
	return c.id
}

func (c *Calendar) GetName() string {
	return c.name
}

func (c *Calendar) GetDescription() string {
	return c.description
}

func (c *Calendar) GetVersion() string {
	return c.version
}

func (c *Calendar) GetTimezone() string {
	return c.timezone
}

func (c *Calendar) GetUrl() string {
	return c.url
}

// #endregion

// #region Setters

func (c *Calendar) SetName(name string) {
	c.name = name
}

func (c *Calendar) SetDescription(description string) {
	c.description = description
}

func (c *Calendar) SetVersion(version string) {
	c.version = version
}

func (c *Calendar) SetTimezone(timezone string) {
	c.timezone = timezone
}

func (c *Calendar) SetUrl(url string) {
	c.url = url
}

// #endregion

func (c *Calendar) AddEvent(event Event) {
	c.events = append(c.events, event)
}
