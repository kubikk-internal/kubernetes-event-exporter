package sinks

import (
	"bytes"
	"encoding/json"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/resmoio/kubernetes-event-exporter/pkg/kube"
)

func GetString(event *kube.EnhancedEvent, text string) (string, error) {
	tmpl, err := template.New("template").Funcs(sprig.TxtFuncMap()).Parse(text)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, event)
	if err != nil {
		return "", err
	}

	// Try to unmarshal if it's a JSON string
	unescaped := buf.String()
	var parsedMessage interface{}
	if json.Unmarshal([]byte(unescaped), &parsedMessage) == nil {
		// Return unmarshaled JSON string
		if marshaled, err := json.Marshal(parsedMessage); err == nil {
			return string(marshaled), nil
		}
	}

	return unescaped, nil
}

func convertLayoutTemplate(layout map[string]interface{}, ev *kube.EnhancedEvent) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for key, value := range layout {
		m, err := convertTemplate(value, ev)
		if err != nil {
			return nil, err
		}
		result[key] = m
	}
	return result, nil
}

func convertTemplate(value interface{}, ev *kube.EnhancedEvent) (interface{}, error) {
	switch v := value.(type) {
	case string:
		rendered, err := GetString(ev, v)
		if err != nil {
			return nil, err
		}

		return rendered, nil
	case map[interface{}]interface{}:
		strKeysMap := make(map[string]interface{})
		for k, v := range v {
			res, err := convertTemplate(v, ev)
			if err != nil {
				return nil, err
			}
			// TODO: It's a bit dangerous
			strKeysMap[k.(string)] = res
		}
		return strKeysMap, nil
	case map[string]interface{}:
		strKeysMap := make(map[string]interface{})
		for k, v := range v {
			res, err := convertTemplate(v, ev)
			if err != nil {
				return nil, err
			}
			strKeysMap[k] = res
		}
		return strKeysMap, nil
	case []interface{}:
		listConf := make([]interface{}, len(v))
		for i := range v {
			t, err := convertTemplate(v[i], ev)
			if err != nil {
				return nil, err
			}
			listConf[i] = t
		}
		return listConf, nil
	}
	return nil, nil
}

func serializeEventWithLayout(layout map[string]interface{}, ev *kube.EnhancedEvent) ([]byte, error) {
	var toSend []byte
	if layout != nil {
		res, err := convertLayoutTemplate(layout, ev)
		if err != nil {
			return nil, err
		}

		// Check if 'message' exists and is a string
		if rawMessage, ok := res["message"].(string); ok {
			var unmarshaledMessage interface{}
			if err := json.Unmarshal([]byte(rawMessage), &unmarshaledMessage); err == nil {
				// If unmarshal succeeds, replace the message with parsed JSON
				res["message"] = unmarshaledMessage
			}
		}

		toSend, err = json.Marshal(res)
		if err != nil {
			return nil, err
		}
	} else {
		toSend = ev.ToJSON()
	}
	return toSend, nil
}
