package customerror

import (
	"errors"
	"gorm.io/gorm"
)

func SqlErrorMap(err error) *CustomError {
	errorsMap := map[error]*CustomError{
		gorm.ErrRecordNotFound:          NewCustomError("Ресурс не найден в базе данных.", 404, err),
		gorm.ErrDuplicatedKey:           NewCustomError("Такой ресурс уже существует.", 409, err),
		gorm.ErrForeignKeyViolated:      NewCustomError("Нарушение целостности: связанный ресурс не существует.", 409, err),
		gorm.ErrCheckConstraintViolated: NewCustomError("Данные не прошли проверку ограничений.", 400, err),
		gorm.ErrInvalidData:             NewCustomError("Переданы некорректные данные для записи.", 400, err),
		gorm.ErrEmptySlice:              NewCustomError("Передан пустой список данных.", 400, err),
		gorm.ErrInvalidDB:               NewCustomError("Ошибка конфигурации базы данных.", 500, err),
		gorm.ErrInvalidTransaction:      NewCustomError("Ошибка транзакции.", 500, err),
	}

	for target, customErr := range errorsMap {
		if errors.Is(err, target) {
			return customErr
		}
	}
	return NewCustomError("Внутренняя ошибка сервера при работе с БД", 500, err)
}
