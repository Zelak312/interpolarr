package main

type WsBaseMessage struct {
	Type string `json:"type"`
}

type WsWorkerProgress struct {
	WsBaseMessage
	WorkerID int     `json:"workerId"`
	Step     string  `json:"step"`
	Progress float64 `json:"progress"`
	Video    *Video  `json:"video"`
}

type WsQeueuUpdate struct {
	WsBaseMessage
	Videos []Video `json:"videos"`
}
