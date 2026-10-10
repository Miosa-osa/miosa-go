package miosa

import (
	"bytes"
	"encoding/json"
)

// The egress routes answer {"data": [...]} for lists and {"data": {...}} for a
// single record. The envelope types model both with a pointer and a slice
// field; these decoders route an array under "data" to the slice so a list
// response decodes.

// splitDataArray removes a top-level array "data" member from a JSON object
// and returns the remaining object plus that array. When "data" is absent or
// not an array the input comes back unchanged with a nil array.
func splitDataArray(b []byte) ([]byte, json.RawMessage) {
	var m map[string]json.RawMessage
	if json.Unmarshal(b, &m) != nil {
		return b, nil
	}
	raw, ok := m["data"]
	if !ok || !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("[")) {
		return b, nil
	}
	delete(m, "data")
	rest, err := json.Marshal(m)
	if err != nil {
		return b, nil
	}
	return rest, raw
}

// UnmarshalJSON implements json.Unmarshaler.
func (e *secretEnvelope) UnmarshalJSON(b []byte) error {
	type plain secretEnvelope
	rest, arr := splitDataArray(b)
	if err := json.Unmarshal(rest, (*plain)(e)); err != nil {
		return err
	}
	if arr != nil {
		return json.Unmarshal(arr, &e.Secrets)
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (e *bindingEnvelope) UnmarshalJSON(b []byte) error {
	type plain bindingEnvelope
	rest, arr := splitDataArray(b)
	if err := json.Unmarshal(rest, (*plain)(e)); err != nil {
		return err
	}
	if arr != nil {
		return json.Unmarshal(arr, &e.Bindings)
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (e *policyEnvelope) UnmarshalJSON(b []byte) error {
	type plain policyEnvelope
	rest, arr := splitDataArray(b)
	if err := json.Unmarshal(rest, (*plain)(e)); err != nil {
		return err
	}
	if arr != nil {
		return json.Unmarshal(arr, &e.Policies)
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (e *ruleEnvelope) UnmarshalJSON(b []byte) error {
	type plain ruleEnvelope
	rest, arr := splitDataArray(b)
	if err := json.Unmarshal(rest, (*plain)(e)); err != nil {
		return err
	}
	if arr != nil {
		return json.Unmarshal(arr, &e.Rules)
	}
	return nil
}
