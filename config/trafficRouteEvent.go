package config

import (
	mvccpb "go.etcd.io/etcd/api/v3/mvccpb"
	etcdLib "go.etcd.io/etcd/client/v3"
)

type TrafficRouteEventType int

const (
	TrafficRouteEventTypePut TrafficRouteEventType = iota
	TrafficRouteEventTypeDelete
)

func NewTrafficRouteEventTypeFrom(eventType mvccpb.Event_EventType) TrafficRouteEventType {
	switch eventType {
	case mvccpb.PUT:
		return TrafficRouteEventTypePut
	case mvccpb.DELETE:
		return TrafficRouteEventTypeDelete
	}
	panic("wrong eventType")
}

type TrafficRouteEvent struct {
	Type TrafficRouteEventType
	KVs  []*mvccpb.KeyValue
}

func NewTrafficRouteEvent(Type TrafficRouteEventType, KVs []*mvccpb.KeyValue) *TrafficRouteEvent {
	event := &TrafficRouteEvent{
		Type,
		KVs,
	}
	return event
}
func NewTrafficRouteEventsFrom(events []*etcdLib.Event) []*TrafficRouteEvent {
	var eMap = map[TrafficRouteEventType]*TrafficRouteEvent{}
	for _, event := range events {
		switch event.Type {
		case etcdLib.EventTypePut:
			putEvent := eMap[TrafficRouteEventTypePut]
			if putEvent == nil {
				putEvent = NewTrafficRouteEvent(TrafficRouteEventTypePut, nil)
			}
			putEvent.KVs = append(putEvent.KVs, event.Kv)
		case etcdLib.EventTypeDelete:
			delEvent := eMap[TrafficRouteEventTypeDelete]
			delEvent.KVs = append(delEvent.KVs, event.Kv)
		}
	}
	var eArr []*TrafficRouteEvent
	for _, val := range eMap {
		eArr = append(eArr, val)
	}
	return eArr
}
