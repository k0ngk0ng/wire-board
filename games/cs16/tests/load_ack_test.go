package cs16_test

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestLoadQueueExpiryClassification(t *testing.T) {
	for _, scenario := range []struct {
		name        string
		body        string
		performance bool
		allowed     bool
	}{
		{"correctness-expiry", `{"type":"ack","status":400,"error":"操作排队已过期","durable":false,"nonce":"input-1"}`, false, true},
		{"performance-expiry", `{"type":"ack","status":400,"error":"操作排队已过期","durable":false,"nonce":"input-1"}`, true, false},
		{"stale-view", `{"type":"ack","status":409,"error":"局面已更新，请根据最新画面重试","durable":false,"nonce":"input-1"}`, false, false},
		{"unknown-error", `{"type":"ack","status":400,"error":"未知错误","durable":false,"nonce":"input-1"}`, false, false},
		{"successful", `{"type":"ack","status":200,"error":"操作排队已过期","durable":false,"nonce":"input-1"}`, false, false},
		{"durable", `{"type":"ack","status":400,"error":"操作排队已过期","durable":true,"nonce":"input-1"}`, false, false},
		{"missing-durable", `{"type":"ack","status":400,"error":"操作排队已过期","nonce":"input-1"}`, false, false},
		{"wrong-type", `{"type":"game","status":400,"error":"操作排队已过期","durable":false,"nonce":"input-1"}`, false, false},
		{"missing-nonce", `{"type":"ack","status":400,"error":"操作排队已过期","durable":false}`, false, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			var answer map[string]any
			decoder := json.NewDecoder(bytes.NewBufferString(scenario.body))
			decoder.UseNumber()
			if err := decoder.Decode(&answer); err != nil {
				t.Fatal(err)
			}
			if expiredLoadInput(answer, scenario.performance) != scenario.allowed {
				t.Fatal("unexpected queue expiry classification", scenario.body)
			}
		})
	}
}
