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

package api

// FetchSeriesRequest contains all the information to fetch a single series of metric
// from a backend.
type FetchSeriesRequest struct {
	Metric       TaggedMetric // metric to fetch.
	SampleMethod SampleMethod // up/downsampling behavior.
	Timerange    Timerange    // time range to fetch data from.
	Api          API          // an API instance.
}

// Backend describes how to fetch time-series data from a given backend.
type Backend interface {

	// FetchSingleSeries should return an instance of BackendError
	FetchSingleSeries(request FetchSeriesRequest) (Timeseries, error)
}

type BackendErrorCode int

const (
	TimeoutError BackendErrorCode = iota + 1
	InvalidSeriesError
)

type BackendError struct {
	Metric TaggedMetric
	Code   BackendErrorCode
}
