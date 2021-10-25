package rules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type httpCallbackListener struct {
	hookURL string
	logger  *zap.Logger
}

func (htcbl httpCallbackListener) callbackDone(ruleID string, attributes extendedAttributes) {
	attributeMap := make(map[string]string)
	for _, k := range attributes.names() {
		attributeMap[k] = *attributes.GetAttribute(k)
	}
	postObj := callbackEvent{
		RuleID:     ruleID,
		Attributes: attributeMap,
	}
	postBody, err := json.Marshal(postObj)
	if err != nil {
		htcbl.logger.Error("Error marshaling body for callback done webhook", zap.Error(err))
		return
	}
	postBuffer := bytes.NewBuffer(postBody)

	resp, err := http.Post(htcbl.hookURL, "application/json", postBuffer)

	if err != nil {
		htcbl.logger.Error("Error sending request to callback done webhook", zap.Error(err))
		return
	}
	htcbl.logger.Info("Request sent to callback done webhook", zap.Int("status", resp.StatusCode), zap.String("data", string(postBody)))
}

type callbackEvent struct {
	RuleID     string            `json:"ruleID"`
	Attributes map[string]string `json:"attributes"`
}

func NewHTTPCallbackHander() HTTPCallbackHandler {
	return HTTPCallbackHandler{
		// The buffer size is arbitrary and not relevant in
		// production settings, since this should only
		// be used for integration tests.
		events: make(chan callbackEvent, 1000),
	}
}

// HTTPCallbackHandler instances can be used to get immediate confirmation that a callback was executed
// when perfoming integration testing. Not for production use.
type HTTPCallbackHandler struct {
	events chan callbackEvent
}

func (htcbh HTTPCallbackHandler) HandleRequest(w http.ResponseWriter, req *http.Request) {
	defer func() {
		_ = req.Body.Close()
	}()
	decoder := json.NewDecoder(req.Body)
	var event callbackEvent
	err := decoder.Decode(&event)
	if err != nil {
		fmt.Printf("Error decoding event: %s\n", err.Error())
	} else {
		htcbh.events <- event
	}
}

// WaitForCallback returns a nil error if the callback was executed with the given ruleID and attributes.
func (htcbh HTTPCallbackHandler) WaitForCallback(ctx context.Context, ruleID string, attributes map[string]string) error {
	select {
	case event := <-htcbh.events:
		if event.RuleID != ruleID || !reflect.DeepEqual(event.Attributes, attributes) {
			// Try again
			return htcbh.WaitForCallback(ctx, ruleID, attributes)
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
