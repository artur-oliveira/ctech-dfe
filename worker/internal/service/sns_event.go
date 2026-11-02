package service

import (
	"encoding/json"
	"fmt"
)

// snsEvent is the Lambda event SNS sends to a Lambda subscription. Raw message
// delivery does not exist for the Lambda protocol: the published body always
// arrives as Records[].Sns.Message, inside this envelope.
type snsEvent struct {
	Records []struct {
		Sns struct {
			MessageID string `json:"MessageId"`
			Message   string `json:"Message"`
		} `json:"Sns"`
	} `json:"Records"`
}

// SNSMessage is one published body plus its SNS id (for logs).
type SNSMessage struct {
	ID   string
	Body string
}

// SNSMessages extracts the published bodies of an SNS Lambda event. A payload
// that is not an SNS event returns an error; an SNS event with no records
// returns none.
func SNSMessages(raw json.RawMessage) ([]SNSMessage, error) {
	var ev snsEvent
	if err := json.Unmarshal(raw, &ev); err != nil {
		return nil, fmt.Errorf("parse SNS event: %w", err)
	}
	out := make([]SNSMessage, 0, len(ev.Records))
	for _, r := range ev.Records {
		out = append(out, SNSMessage{ID: r.Sns.MessageID, Body: r.Sns.Message})
	}
	return out, nil
}
