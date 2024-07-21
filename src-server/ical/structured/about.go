// Package `structured` contains iCalendar properties that are not part following
// the standard `key:value` format, but are instead `prop;param=value`.
//
// For example, the `ATTENDEE` property is defined as `ATTENDEE;CN=Name:mailto:email@example.com`,
// but the `CN` and `mailto` parameters are not part of the key.
//
// To create a new property, use the `NewAttendee` or `NewAlarm` functions, then
package structured
