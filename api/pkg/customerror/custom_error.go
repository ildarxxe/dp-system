package customerror

import "fmt"

type CustomError struct {
	Message       string `json:"message"`
	Status        int    `json:"status"`
	OriginalError error  `json:"original_error"`
}

func NewCustomError(message string, status int, err error) *CustomError {
	return &CustomError{Message: message, Status: status, OriginalError: err}
}

func (e *CustomError) Error() string {
	if e.Unwrap() != nil {
		return fmt.Sprintf("[Custom Error] Статус: %d, Сообщение: %s, Ошибка: %v", e.Status, e.Message, e.OriginalError)
	}
	return fmt.Sprintf("[Custom Error] Статус: %d, Сообщение: %s", e.Status, e.Message)
}

func (e *CustomError) Unwrap() error {
	return e.OriginalError
}
