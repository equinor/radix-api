package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/assert"
)

func Test_Event_Marshal(t *testing.T) {
	event := Event{
		LastTimestamp:   strfmt.DateTime(time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)),
		ObjectKind:      "akind",
		ObjectNamespace: "anamespace",
		ObjectName:      "aname",
		Type:            "atype",
		Reason:          "areason",
		Message:         "amessage",
		Count:           2,
	}
	expected := "{\"lastTimestamp\":\"2020-01-02T03:04:05.000Z\",\"count\":2,\"objectKind\":\"akind\",\"objectNamespace\":\"anamespace\",\"objectName\":\"aname\",\"type\":\"atype\",\"reason\":\"areason\",\"message\":\"amessage\"}"
	eventbytes, err := json.Marshal(event)
	assert.Nil(t, err)
	eventjson := string(eventbytes)
	assert.Equal(t, expected, eventjson)
}
