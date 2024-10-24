package main

type WsBaseMessage struct {
	Type string `json:"type"`
}

type WsWorkerProgress struct {
	WsBaseMessage
	Worker
}

type WsQeueuUpdate struct {
	WsBaseMessage
	Videos []Video `json:"videos"`
}
