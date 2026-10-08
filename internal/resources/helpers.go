package resources

import "encoding/json"

func jsonUnmarshalString(s string, v interface{}) error {
	return json.Unmarshal([]byte(s), v)
}

func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
