package bybit

import (
	"encoding/json"
	"strconv"
)

type FlexibleInt64 int64

func (f *FlexibleInt64) UnmarshalJSON(data []byte) error {
	var number int64

	if err := json.Unmarshal(data, &number); err == nil {
		*f = FlexibleInt64(number)

		return nil
	}

	var stringValue string

	if err := json.Unmarshal(data, &stringValue); err == nil {
		number, err := strconv.ParseInt(stringValue, 10, 64)
		if err != nil {
			return err
		}

		*f = FlexibleInt64(number)

		return nil
	}

	return nil
}
