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

// program which takes
// - a rule file
// - a sample list of metrics
// and sees how well the rule performs against the metrics.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"sort"

	"github.com/square/metrics/api"
	"github.com/square/metrics/internal"
	"github.com/square/metrics/main/common"
)

var (
	metricsFile      = flag.String("metrics-file", "", "Location of YAML configuration file.")
	unmatchedFile    = flag.String("unmatched-file", "", "location of metrics list to output unmatched transformations.")
	insertToDatabase = flag.Bool("insert-to-db", false, "If true, insert rows to database.")
)

func readRule(filename string) *internal.RuleSet {
	file, err := os.Open(filename)
	if err != nil {
		common.ExitWithMessage("No rule file")
	}
	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		common.ExitWithMessage("Cannot read the rule YAML")
	}
	rule, err := internal.LoadYAML(bytes)
	if err != nil {
		common.ExitWithMessage("Cannot parse Rule file")
	}
	return &rule
}

// Statistics represents the aggregated result of rules
// after running through the test file.
type Statistics struct {
	perMetric map[api.MetricKey]PerMetricStatistics
	matched   int // number of matched rows
	unmatched int // number of unmatched rows
}

// PerMetricStatistics represents per-metric result of rules
// after running through the test file.
type PerMetricStatistics struct {
	matched          int // number of matched rows
	reverseSuccess   int // number of reversed entries
	reverseError     int // number of incorrectly reversed entries.
	reverseIncorrect int // number of incorrectly reversed entries.
}

func main() {
	flag.Parse()
	common.SetupLogger()

	config := common.LoadConfig()

	ruleset := readRule(config.API.ConversionRulesPath)
	metricFile, err := os.Open(*metricsFile)
	if err != nil {
		common.ExitWithMessage("No metric file.")
	}
	scanner := bufio.NewScanner(metricFile)
	apiInstance := common.NewAPI(config.API)
	var output *os.File
	if *unmatchedFile != "" {
		output, err = os.Create(*unmatchedFile)
		if err != nil {
			common.ExitWithMessage(fmt.Sprintf("Error creating the output file: %s", err.Error()))
		}
	}
	stat := run(ruleset, scanner, apiInstance, output)
	report(stat)
}

func run(ruleset *internal.RuleSet, scanner *bufio.Scanner, apiInstance api.API, unmatched *os.File) Statistics {
	stat := Statistics{
		perMetric: make(map[api.MetricKey]PerMetricStatistics),
	}
	for scanner.Scan() {
		input := scanner.Text()
		converted, matched := ruleset.MatchRule(input)
		if matched {
			stat.matched++
			perMetric := stat.perMetric[converted.MetricKey]
			perMetric.matched++
			reversed, err := ruleset.ToGraphiteName(converted)
			if *insertToDatabase {
				apiInstance.AddMetric(converted)
			}
			if err != nil {
				perMetric.reverseError++
			} else if string(reversed) != input {
				perMetric.reverseIncorrect++
			} else {
				perMetric.reverseSuccess++
			}
			stat.perMetric[converted.MetricKey] = perMetric
		} else {
			stat.unmatched++
			if unmatched != nil {
				unmatched.WriteString(input)
				unmatched.WriteString("\n")
			}
		}
	}
	return stat
}

func report(stat Statistics) {
	total := stat.matched + stat.unmatched
	fmt.Printf("Processed %d entries\n", total)
	fmt.Printf("Matched:   %d\n", stat.matched)
	fmt.Printf("Unmatched: %d\n", stat.unmatched)
	fmt.Printf("Per-rule statistics\n")
	rowformat := "%-60s %7d %7d %7d %7d\n"
	headformat := "%-60s %7s %7s %7s %7s\n"
	fmt.Printf(headformat, "name", "match", "rev-suc", "rev-err", "rev-fail")
	sortedKeys := make([]string, len(stat.perMetric))
	index := 0
	for key := range stat.perMetric {
		sortedKeys[index] = string(key)
		index++
	}
	sort.Strings(sortedKeys)
	for _, key := range sortedKeys {
		perMetric := stat.perMetric[api.MetricKey(key)]
		fmt.Printf(rowformat,
			string(key),
			perMetric.matched,
			perMetric.reverseSuccess,
			perMetric.reverseError,
			perMetric.reverseIncorrect,
		)
	}
}
