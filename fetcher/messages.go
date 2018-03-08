package fetcher

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// FetchTopic is the NSQ topic to which to publish FetchMessage messages.
const FetchTopic = "fetch"

// FetchMessage is an envelope message published to FetchTopic.
// This is a magical struct that when JSON serialized will take the form:
// `{"t": "character", "d": {"id": "12345"}}`
type FetchMessage struct{ Job }

// MarshalJSON marshals the message to JSON.
func (msg FetchMessage) MarshalJSON() ([]byte, error) {
	if msg.Job == nil {
		return nil, errors.New("can't marshal an empty FetchMessage")
	}
	return json.Marshal(struct {
		Type string      `json:"t"`
		Data interface{} `json:"d"`
	}{
		msg.Job.Type(),
		msg.Job,
	})
}

// UnmarshalJSON unmarshals JSON data.
func (msg *FetchMessage) UnmarshalJSON(data []byte) error {
	var d struct {
		Type string          `json:"t"`
		Data json.RawMessage `json:"d"`
	}
	if err := json.Unmarshal(data, &d); err != nil {
		return err
	}
	msg.Job = jobIndex[d.Type]()
	return json.Unmarshal(d.Data, msg.Job)
}
