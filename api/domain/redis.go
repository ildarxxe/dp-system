package domain

type RedisMessage struct {
	Message string `json:"message"`
	TaskID  uint64 `json:"task_id"`
}
