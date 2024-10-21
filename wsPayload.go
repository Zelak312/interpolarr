package main

type WsBaseMessage struct {
	Type string `json:"type"`
}

type WsWorkerProgress struct {
	WsBaseMessage
	WorkerID string  `json:"workerId"`
	Step     string  `json:"step"`
	Progress float64 `json:"progress"`
}
