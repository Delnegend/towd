package ical

import "github.com/google/uuid"

type Calendar struct {
	id          string
	name        string
	description string
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

func (c *Calendar) SetUrl(url string) {
	c.url = url
}

// #endregion

func (c *Calendar) AddEvent(event Event) {
	c.events = append(c.events, event)
}
