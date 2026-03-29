package utils

import "encoding/json"

func Convert(input interface{}, output interface{}) {
	b, _ := json.Marshal(input)
	json.Unmarshal(b, output)
}
