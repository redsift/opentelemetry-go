// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package propagators

import (
	"context"
	"net/url"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/internal/baggage"
	"go.opentelemetry.io/otel/label"
)

// Temporary header name until W3C finalizes format.
// https://github.com/open-telemetry/opentelemetry-specification/blob/18b2752ebe6c7f0cdd8c7b2bcbdceb0ae3f5ad95/specification/correlationcontext/api.md#header-name
const baggageHeader = "otcorrelations"

// Baggage is a propagator that supports the W3C Baggage format.
//
// This propagates user-defined baggage associated with a trace. The complete
// specification is defined at https://w3c.github.io/baggage/.
type Baggage struct{}

var _ otel.TextMapPropagator = Baggage{}

// Inject sets baggage key-values from ctx into the carrier.
func (b Baggage) Inject(ctx context.Context, carrier otel.TextMapCarrier) {
	baggageMap := baggage.MapFromContext(ctx)
	firstIter := true
	var headerValueBuilder strings.Builder
	baggageMap.Foreach(func(kv label.KeyValue) bool {
		if !firstIter {
			headerValueBuilder.WriteRune(',')
		}
		firstIter = false
		headerValueBuilder.WriteString(url.QueryEscape(strings.TrimSpace((string)(kv.Key))))
		headerValueBuilder.WriteRune('=')
		headerValueBuilder.WriteString(url.QueryEscape(strings.TrimSpace(kv.Value.Emit())))
		return true
	})
	if headerValueBuilder.Len() > 0 {
		headerString := headerValueBuilder.String()
		carrier.Set(baggageHeader, headerString)
	}
}

// Extract returns a copy of parent with the baggage from the carrier added.
func (b Baggage) Extract(parent context.Context, carrier otel.TextMapCarrier) context.Context {
	bVal := carrier.Get(baggageHeader)
	if bVal == "" {
		return parent
	}

	baggageValues := strings.Split(bVal, ",")
	keyValues := make([]label.KeyValue, 0, len(baggageValues))
	for _, baggageValue := range baggageValues {
		valueAndProps := strings.Split(baggageValue, ";")
		if len(valueAndProps) < 1 {
			continue
		}
		nameValue := strings.Split(valueAndProps[0], "=")
		if len(nameValue) < 2 {
			continue
		}
		name, err := url.QueryUnescape(nameValue[0])
		if err != nil {
			continue
		}
		trimmedName := strings.TrimSpace(name)
		value, err := url.QueryUnescape(nameValue[1])
		if err != nil {
			continue
		}
		trimmedValue := strings.TrimSpace(value)

		// TODO (skaris): properties defiend https://w3c.github.io/correlation-context/, are currently
		// just put as part of the value.
		var trimmedValueWithProps strings.Builder
		trimmedValueWithProps.WriteString(trimmedValue)
		for _, prop := range valueAndProps[1:] {
			trimmedValueWithProps.WriteRune(';')
			trimmedValueWithProps.WriteString(prop)
		}

		keyValues = append(keyValues, label.String(trimmedName, trimmedValueWithProps.String()))
	}

	if len(keyValues) > 0 {
		// Only update the context if valid values were found
		return baggage.ContextWithMap(parent, baggage.NewMap(baggage.MapUpdate{
			MultiKV: keyValues,
		}))
	}

	return parent
}

// Fields returns the keys who's values are set with Inject.
func (b Baggage) Fields() []string {
	return []string{baggageHeader}
}
