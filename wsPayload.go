package main

type WsBaseMessage struct {
	Type string `json:"type"`
}

type WsWorkerProgress struct {
	WsBaseMessage
	WorkerID int     `json:"workerId"`
	Step     string  `json:"step"`
	Progress float64 `json:"progress"`
}
