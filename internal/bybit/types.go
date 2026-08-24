package bybit

import (
	"encoding/json"
	"strconv"
)

// FlexibleInt64 нужен потому,
// что Bybit в разных сообщениях может присылать timestamp:
//
//	1787588392280
//
// или:
//
//	"1787588392280"
//
// Обычный int64 второй вариант не распарсит.
type FlexibleInt64 int64

// UnmarshalJSON позволяет поддерживать оба варианта.
//
// Это хороший пример собственного JSON-типа в Go.
func (f *FlexibleInt64) UnmarshalJSON(data []byte) error {
	// Сначала пробуем число.
	var number int64

	if err := json.Unmarshal(data, &number); err == nil {
		*f = FlexibleInt64(number)

		return nil
	}

	// Если не число — пробуем строку.
	var stringValue string

	if err := json.Unmarshal(data, &stringValue); err == nil {
		number, err := strconv.ParseInt(
			stringValue,
			10,
			64,
		)

		if err != nil {
			return err
		}

		*f = FlexibleInt64(number)

		return nil
	}

	// Сохраняем старое поведение:
	// если значение вообще нельзя распознать,
	// не падаем на unmarshalling всего сообщения.
	return nil
}
