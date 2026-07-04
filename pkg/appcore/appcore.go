// Package appcore is the gomobile-bindable facade over pkg/collection, used
// to build the iOS/macOS postcard viewer app's Postcards.xcframework.
//
// Every exported type and method here must satisfy gomobile's binding
// restrictions: parameters/results may only be string, bool, numeric types,
// []byte, error, or pointers to exported structs declared in this package.
// Richer data (collection.CardSummary, collection.SearchResult,
// types.Metadata, ...) therefore always crosses the bridge as a JSON string
// rather than as Go structs or slices.
package appcore

import (
	"encoding/json"
	"fmt"
)

func marshalJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("encoding JSON: %w", err)
	}
	return string(b), nil
}
