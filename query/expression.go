// Copyright 2015 Square Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package query

import (
	"errors"
	"fmt"

	"github.com/square/metrics/api"
	"github.com/square/metrics/function"
)

// Implementations
// ===============

func (expr durationExpression) Evaluate(context function.EvaluationContext) (function.Value, error) {
	return function.DurationValue(expr.duration), nil
}

func (expr scalarExpression) Evaluate(context function.EvaluationContext) (function.Value, error) {
	return function.ScalarValue(expr.value), nil
}

func (expr stringExpression) Evaluate(context function.EvaluationContext) (function.Value, error) {
	return function.StringValue(expr.value), nil
}

func (expr *metricFetchExpression) Evaluate(context function.EvaluationContext) (function.Value, error) {
	// Merge predicates appropriately
	var predicate api.Predicate
	if context.Predicate == nil && expr.predicate == nil {
		predicate = api.TruePredicate
	} else if context.Predicate == nil {
		predicate = expr.predicate
	} else if expr.predicate == nil {
		predicate = context.Predicate
	} else {
		predicate = &andPredicate{[]api.Predicate{expr.predicate, context.Predicate}}
	}

	metricTagSets, err := context.API.GetAllTags(api.MetricKey(expr.metricName))
	if err != nil {
		return nil, err
	}
	filtered := applyPredicates(metricTagSets, predicate)

	ok := context.FetchLimit.Consume(len(filtered))

	if !ok {
		return nil, errors.New("fetch limit exceeded: too many series to fetch")
	}

	metrics := make([]api.TaggedMetric, len(filtered))
	for i := range metrics {
		metrics[i] = api.TaggedMetric{api.MetricKey(expr.metricName), filtered[i]}
	}

	serieslist, err := context.MultiBackend.FetchMultipleSeries(
		api.FetchMultipleRequest{
			metrics,
			context.SampleMethod,
			context.Timerange,
			context.API,
			context.Cancellable,
			context.Profiler,
		},
	)

	if err != nil {
		return nil, err
	}

	serieslist.Name = expr.metricName

	return function.SeriesListValue(serieslist), nil
}

func (expr *functionExpression) Evaluate(context function.EvaluationContext) (function.Value, error) {
	fun, ok := context.Registry.GetFunction(expr.functionName)
	if !ok {
		return nil, SyntaxError{expr.functionName, fmt.Sprintf("no such function %s", expr.functionName)}
	}

	return fun.Evaluate(context, expr.arguments, expr.groupBy)
}

// Auxiliary functions
// ===================

func applyPredicates(tagSets []api.TagSet, predicate api.Predicate) []api.TagSet {
	output := []api.TagSet{}
	for _, ts := range tagSets {
		if predicate.Apply(ts) {
			output = append(output, ts)
		}
	}
	return output
}

// evaluateExpressions evaluates all provided Expressions in the
// EvaluationContext. If any evaluations error, evaluateExpressions will
// propagate that error. The resulting SeriesLists will be in an order
// corresponding to the provided Expresesions.
func evaluateExpressions(context function.EvaluationContext, expressions []function.Expression) ([]function.Value, error) {
	if len(expressions) == 0 {
		return []function.Value{}, nil
	}
	results := make([]function.Value, len(expressions))
	for i, expr := range expressions {
		result, err := expr.Evaluate(context)
		if err != nil {
			return nil, err
		}
		results[i] = result
	}
	return results, nil
}
