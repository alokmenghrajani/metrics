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

package internal

import (
	"testing"

	"github.com/square/metrics/api"
	"github.com/square/metrics/assert"
)

func checkRuleErrorCode(a assert.Assert, err error, expected RuleErrorCode) {
	a = a.Stack(1)
	if err == nil {
		a.Errorf("No error provided.")
		return
	}
	casted, ok := err.(RuleError)
	if !ok {
		a.Errorf("Invalid Error type: %s", err.Error())
		return
	}
	a.EqInt(int(casted.Code()), int(expected))
}

func checkConversionErrorCode(t *testing.T, err error, expected ConversionErrorCode) {
	casted, ok := err.(ConversionError)
	if !ok {
		t.Errorf("Invalid Error type")
		return
	}
	a := assert.New(t)
	a.EqInt(int(casted.Code()), int(expected))
}

func TestCompile_Good(t *testing.T) {
	a := assert.New(t)
	_, err := Compile(RawRule{
		Pattern:          "prefix.%foo%",
		MetricKeyPattern: "test-metric",
	})
	a.CheckError(err)
}

func TestCompile_Error(t *testing.T) {
	for _, test := range []struct {
		rawRule      RawRule
		expectedCode RuleErrorCode
	}{
		{RawRule{Pattern: "prefix.%foo%", MetricKeyPattern: ""}, InvalidMetricKey},
		{RawRule{Pattern: "prefix.%foo%abc%", MetricKeyPattern: "test-metric"}, InvalidPattern},
		{RawRule{Pattern: "", MetricKeyPattern: "test-metric"}, InvalidPattern},
		{RawRule{Pattern: "prefix.%foo%.%foo%", MetricKeyPattern: "test-metric"}, InvalidPattern},
		{RawRule{Pattern: "prefix.%foo%.abc.%%", MetricKeyPattern: "test-metric"}, InvalidPattern},
		{RawRule{Pattern: "prefix.%foo%", MetricKeyPattern: "test-metric", Regex: map[string]string{"foo": "(bar)"}}, InvalidCustomRegex},
		{RawRule{Pattern: "prefix.%foo%", MetricKeyPattern: "test-metric", Regex: map[string]string{"foo": "(bar)"}}, 0},
	} {
		_, err := Compile(test.rawRule)
		a := assert.New(t).Contextf("%s", test.rawRule.Pattern)
		checkRuleErrorCode(a, err, test.expectedCode)
	}
}

func TestMatchRule_Simple(t *testing.T) {
	a := assert.New(t)
	rule, err := Compile(RawRule{
		Pattern:          "prefix.%foo%",
		MetricKeyPattern: "test-metric",
	})
	a.CheckError(err)

	_, matches := rule.MatchRule("")
	if matches {
		t.Errorf("Unexpected matching")
	}
	matcher, matches := rule.MatchRule("prefix.abc")
	if !matches {
		t.Errorf("Expected matching but didn't occur")
	}
	a.EqString(string(matcher.MetricKey), "test-metric")
	a.EqString(matcher.TagSet["foo"], "abc")

	_, matches = rule.MatchRule("prefix.abc.def")
	if matches {
		t.Errorf("Unexpected matching")
	}
}

func TestMatchRule_FilterTag(t *testing.T) {
	a := assert.New(t)
	rule, err := Compile(RawRule{
		Pattern:          "prefix.%foo%.%bar%",
		MetricKeyPattern: "test-metric.%bar%",
	})
	a.CheckError(err)
	originalName := "prefix.fooValue.barValue"
	matcher, matched := rule.MatchRule(originalName)
	if !matched {
		t.Errorf("Expected matching but didn't occur")
		return
	}
	a.EqString(string(matcher.MetricKey), "test-metric.barValue")
	a.Eq(matcher.TagSet, api.TagSet(map[string]string{"foo": "fooValue"}))
	// perform the reverse.
	reversed, err := rule.ToGraphiteName(matcher)
	a.CheckError(err)
	a.EqString(string(reversed), originalName)
}

func TestMatchRule_CustomRegex(t *testing.T) {
	a := assert.New(t)
	regex := make(map[string]string)
	regex["name"] = "[a-z]+"
	regex["shard"] = "[0-9]+"
	rule, err := Compile(RawRule{
		Pattern:          "feed.%name%-shard-%shard%",
		MetricKeyPattern: "test-feed-metric",
		Regex:            regex,
	})
	a.CheckError(err)

	_, matches := rule.MatchRule("")
	if matches {
		t.Errorf("Unexpected matching")
	}
	matcher, matches := rule.MatchRule("feed.feedname-shard-12")
	if !matches {
		t.Errorf("Expected matching but didn't occur")
	}
	a.EqString(string(matcher.MetricKey), "test-feed-metric")
	a.EqString(matcher.TagSet["name"], "feedname")
	a.EqString(matcher.TagSet["shard"], "12")
}

func TestLoadYAML(t *testing.T) {
	a := assert.New(t)
	rawYAML := `
rules:
  -
    pattern: foo.bar.baz.%tag%
    metric_key: abc
    regex: {}
  `
	ruleSet, err := LoadYAML([]byte(rawYAML))
	a.CheckError(err)
	a.EqInt(len(ruleSet.rules), 1)
	a.EqString(string(ruleSet.rules[0].raw.MetricKeyPattern), "abc")
	a.Eq(ruleSet.rules[0].graphitePatternTags, []string{"tag"})
}

func TestLoadYAML_Invalid(t *testing.T) {
	a := assert.New(t)
	rawYAML := `
rules
  -
    pattern: foo.bar.baz.%tag%
    metric_key: abc
    regex: {}
  `
	ruleSet, err := LoadYAML([]byte(rawYAML))
	checkRuleErrorCode(a, err, InvalidYaml)
	a.EqInt(len(ruleSet.rules), 0)
}

func TestToGraphiteName(t *testing.T) {
	a := assert.New(t)
	rule, err := Compile(RawRule{
		Pattern:          "prefix.%foo%",
		MetricKeyPattern: "test-metric",
	})
	a.CheckError(err)
	tm := api.TaggedMetric{
		MetricKey: "test-metric",
		TagSet:    api.ParseTagSet("foo=fooValue"),
	}
	reversed, err := rule.ToGraphiteName(tm)
	a.CheckError(err)
	a.EqString(string(reversed), "prefix.fooValue")
}

func TestToGraphiteName_Error(t *testing.T) {
	a := assert.New(t)
	rule, err := Compile(RawRule{
		Pattern:          "prefix.%foo%",
		MetricKeyPattern: "test-metric",
	})
	a.CheckError(err)
	reversed, err := rule.ToGraphiteName(api.TaggedMetric{
		MetricKey: "test-metric",
		TagSet:    api.ParseTagSet(""),
	})
	checkConversionErrorCode(t, err, MissingTag)
	a.EqString(string(reversed), "")

	reversed, err = rule.ToGraphiteName(api.TaggedMetric{
		MetricKey: "test-metric-foo",
		TagSet:    api.ParseTagSet("foo=fooValue"),
	})
	checkConversionErrorCode(t, err, CannotInterpolate)
	a.EqString(string(reversed), "")
}
