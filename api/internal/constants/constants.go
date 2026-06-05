package constants

type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusUploaded   TaskStatus = "uploaded"
	StatusProcessing TaskStatus = "processing"
	StatusSuccess    TaskStatus = "success"
	StatusFailed     TaskStatus = "failed"
)

type VideoAction string

const (
	ActionCompress VideoAction = "compress"
	ActionToMP4    VideoAction = "mp4"
)
