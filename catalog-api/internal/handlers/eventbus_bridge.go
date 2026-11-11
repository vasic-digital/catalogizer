package handlers

import (
	"catalogizer/internal/eventbus"
	"fmt"
	"sync"

	root_handlers "catalogizer/handlers"

	"go.uber.org/zap"
)

// EventBusBridge subscribes to system eventbus events and forwards them to
// WebSocket clients via the existing root WebSocket handler.
//
// The bridge reads from the eventbus subscription channel in a background
// goroutine and calls wsHandler.BroadcastToClients for each matching event.
// Events are mapped to client-facing message types that the UI already handles
// (notification, media_update, analysis_complete in websocket.ts).
// Publication is non-blocking: BroadcastToClients drops messages for clients
// whose send buffer is full rather than blocking the bridge.
type EventBusBridge struct {
	bus       *eventbus.EventBus
	wsHandler *root_handlers.WebSocketHandler
	logger    *zap.Logger

	subCancel  func()
	closeOnce  sync.Once
	stopCh     chan struct{}
	routingMap map[eventbus.EventType]wsRoute
}

// wsRoute describes how a system event is mapped to a WebSocket message.
type wsRoute struct {
	wsType string        // The "type" field in the client-facing message
	build  func(*eventbus.Event) map[string]interface{}
}

// NewEventBusBridge creates a new bridge that subscribes to the given eventbus
// and forwards relevant events to WebSocket clients.
func NewEventBusBridge(bus *eventbus.EventBus, wsHandler *root_handlers.WebSocketHandler, logger *zap.Logger) *EventBusBridge {
	if logger == nil {
		logger = zap.NewNop()
	}

	b := &EventBusBridge{
		bus:       bus,
		wsHandler: wsHandler,
		logger:    logger,
		stopCh:    make(chan struct{}),
		routingMap: map[eventbus.EventType]wsRoute{
			eventbus.EventScanCompleted: {
				wsType: "notification",
				build: func(evt *eventbus.Event) map[string]interface{} {
					payload, _ := evt.Payload.(map[string]interface{})
					msg := "Scan completed"
					if payload != nil {
						if sr, ok := payload["storage_root"].(string); ok {
							msg = "Scan completed: " + sr
						}
						if fp, ok := payload["files_found"].(int64); ok && fp > 0 {
							msg += " (" + fmt.Sprintf("%d files found", fp) + ")"
						}
					}
					return map[string]interface{}{
						"type": "notification",
						"payload": map[string]interface{}{
							"level":   "success",
							"message": msg,
						},
					}
				},
			},
			eventbus.EventScanFailed: {
				wsType: "notification",
				build: func(evt *eventbus.Event) map[string]interface{} {
					payload, _ := evt.Payload.(map[string]interface{})
					msg := "Scan failed"
					if payload != nil {
						if sr, ok := payload["storage_root"].(string); ok {
							msg = "Scan failed: " + sr
						}
						if errStr, ok := payload["error"].(string); ok {
							msg += ": " + errStr
						}
					}
					return map[string]interface{}{
						"type": "notification",
						"payload": map[string]interface{}{
							"level":   "error",
							"message": msg,
						},
					}
				},
			},
			eventbus.EventScanCancelled: {
				wsType: "notification",
				build: func(evt *eventbus.Event) map[string]interface{} {
					payload, _ := evt.Payload.(map[string]interface{})
					msg := "Scan cancelled"
					if payload != nil {
						if sr, ok := payload["storage_root"].(string); ok {
							msg = "Scan cancelled: " + sr
						}
						if reason, ok := payload["reason"].(string); ok && reason != "" {
							msg += ": " + reason
						}
					}
					return map[string]interface{}{
						"type": "notification",
						"payload": map[string]interface{}{
							"level":   "warning",
							"message": msg,
						},
					}
				},
			},
			eventbus.EventScanStarted: {
				wsType: "scan_started",
				build: func(evt *eventbus.Event) map[string]interface{} {
					payload, _ := evt.Payload.(map[string]interface{})
					msg := map[string]interface{}{
						"type":    "scan_started",
						"payload": payload,
					}
					if payload == nil {
						msg["payload"] = evt.Payload
					}
					return msg
				},
			},
			eventbus.EventEntityCreated: {
				wsType: "media_update",
				build: func(evt *eventbus.Event) map[string]interface{} {
					payload, _ := evt.Payload.(map[string]interface{})
					return map[string]interface{}{
						"type":    "media_update",
						"payload": payload,
					}
				},
			},
			eventbus.EventEntityUpdated: {
				wsType: "media_update",
				build: func(evt *eventbus.Event) map[string]interface{} {
					payload, _ := evt.Payload.(map[string]interface{})
					return map[string]interface{}{
						"type":    "media_update",
						"payload": payload,
					}
				},
			},
			eventbus.EventFileCreated: {
				wsType: "file_created",
				build: func(evt *eventbus.Event) map[string]interface{} {
					return map[string]interface{}{
						"type":    "file_created",
						"payload": evt.Payload,
					}
				},
			},
			eventbus.EventFileModified: {
				wsType: "file_modified",
				build: func(evt *eventbus.Event) map[string]interface{} {
					return map[string]interface{}{
						"type":    "file_modified",
						"payload": evt.Payload,
					}
				},
			},
			eventbus.EventFileDeleted: {
				wsType: "file_deleted",
				build: func(evt *eventbus.Event) map[string]interface{} {
					return map[string]interface{}{
						"type":    "file_deleted",
						"payload": evt.Payload,
					}
				},
			},
		},
	}

	return b
}

// Start subscribes to the eventbus and begins forwarding events to WebSocket clients.
func (b *EventBusBridge) Start() {
	sub := b.bus.SubscribeMultiple(
		eventbus.EventScanCompleted,
		eventbus.EventScanFailed,
		eventbus.EventScanCancelled,
		eventbus.EventScanStarted,
		eventbus.EventFileCreated,
		eventbus.EventFileModified,
		eventbus.EventFileDeleted,
		eventbus.EventFileMoved,
		eventbus.EventEntityCreated,
		eventbus.EventEntityUpdated,
	)
	b.subCancel = sub.Cancel

	b.logger.Info("EventBus bridge started, forwarding events to WebSocket clients")

	go b.forwardLoop(sub)
}

// Stop gracefully stops the bridge and unsubscribes from the eventbus.
func (b *EventBusBridge) Stop() {
	b.closeOnce.Do(func() {
		close(b.stopCh)
		if b.subCancel != nil {
			b.subCancel()
		}
		b.logger.Info("EventBus bridge stopped")
	})
}

// forwardLoop reads events from the subscription channel and forwards them
// to connected WebSocket clients.
func (b *EventBusBridge) forwardLoop(sub *eventbus.Subscription) {
	for {
		select {
		case <-b.stopCh:
			return
		case evt, ok := <-sub.Channel:
			if !ok {
				b.logger.Warn("EventBus subscription channel closed")
				return
			}
			b.forwardEvent(evt)
		}
	}
}

// forwardEvent converts a system event to a WebSocket JSON message and
// broadcasts it to all connected clients.
func (b *EventBusBridge) forwardEvent(evt *eventbus.Event) {
	route, ok := b.routingMap[evt.Type]
	if !ok {
		return // Unknown event type — don't forward
	}

	msg := route.build(evt)
	b.wsHandler.BroadcastToClients(msg)

	b.logger.Debug("Forwarded event to WebSocket clients",
		zap.String("event_type", string(evt.Type)),
		zap.String("ws_type", route.wsType))
}
