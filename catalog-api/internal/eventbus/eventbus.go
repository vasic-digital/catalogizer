// Package eventbus exposes digital.vasic.eventbus types for use within Catalogizer.
//
// This package re-exports the core EventBus, Event, and Subscription types from
// digital.vasic.eventbus as type aliases so all Catalogizer code can reference
// a single import path. No adapters are needed — Go type aliases are transparent
// to the compiler.
//
// Design patterns applied:
//   - Facade: single import point for all event bus types
//   - Observer: publish/subscribe event-driven communication
//   - Mediator: decouples publishers from subscribers
package eventbus

import (
	vasicbus "digital.vasic.eventbus/pkg/bus"
	vasicevent "digital.vasic.eventbus/pkg/event"
	vasicfilter "digital.vasic.eventbus/pkg/filter"
	vasicmw "digital.vasic.eventbus/pkg/middleware"
)

// EventBus provides publish/subscribe for system events.
// Backed by digital.vasic.eventbus/pkg/bus.EventBus.
//
// Publish is non-blocking: each subscriber has a buffered channel (default 1000)
// and trySend uses a configurable PublishTimeout (default 10ms). Slow subscribers
// cause event drops (tracked in Metrics.EventsDropped) rather than blocking the
// publisher, so no additional semaphore is needed.
type EventBus = vasicbus.EventBus

// Config holds configuration for the event bus.
type Config = vasicbus.Config

// Metrics tracks event bus statistics.
type Metrics = vasicbus.Metrics

// Event represents a system event with typed payload.
// Backed by digital.vasic.eventbus/pkg/event.Event.
type Event = vasicevent.Event

// Type represents an event type using dot-notation topics.
// Examples: "scan.completed", "file.created", "metadata.refreshed"
type EventType = vasicevent.Type

// Subscription represents an active subscription to events.
type Subscription = vasicevent.Subscription

// Filter is a function that returns true when an event should be delivered.
type Filter = vasicfilter.Filter

// Middleware is a function that transforms events before delivery.
type Middleware = vasicmw.Middleware

// DefaultConfig returns sensible defaults for a new EventBus.
func DefaultConfig() *Config {
	return vasicbus.DefaultConfig()
}

// New creates a new EventBus with the given configuration.
func New(config *Config) *EventBus {
	return vasicbus.New(config)
}

// NewEvent creates a new event with auto-generated ID, trace ID, and timestamp.
func NewEvent(eventType EventType, source string, payload interface{}) *Event {
	return vasicevent.New(eventType, source, payload)
}

// Catalogizer-specific event types using dot-notation topics.
const (
	EventScanStarted    EventType = "scan.started"
	EventScanCompleted  EventType = "scan.completed"
	EventScanFailed     EventType = "scan.failed"
	EventScanCancelled  EventType = "scan.cancelled"
	EventFileCreated    EventType = "file.created"
	EventFileModified   EventType = "file.modified"
	EventFileDeleted    EventType = "file.deleted"
	EventFileMoved      EventType = "file.moved"
	EventEntityCreated  EventType = "entity.created"
	EventEntityUpdated  EventType = "entity.updated"
	EventMetaRefreshed  EventType = "metadata.refreshed"
	EventCacheEvicted   EventType = "cache.evicted"
	EventSystemStartup  EventType = "system.startup"
	EventSystemShutdown EventType = "system.shutdown"
)
