/*
Copyright 2019 Atos

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/*
Package prometheus provides a Retriever to get monitoring metrics
from a Prometheus TSDB
*/
package prometheus

/*
Example of query:
curl 'localhost:9090/api/v1/query?query=hmm_compute_dbdump_time&time=2019-11-14T16:00:00Z'

Example of query range:
curl 'localhost:9090/api/v1/query_range?query=hmm_compute_dbdump_time&start=2019-11-14T16:00:00Z&end=2019-11-14T17:00:00Z&step=15s'

Example of vector output from Prometheus:

	{
		"status": "success",
		"data": {
			"resultType": "vector",
			"result": [
				{
					"metric": {
						"__name__": "go_memstats_frees_total",
						"instance": "localhost:9090",
						"job": "prometheus"
					},
					"value": [
						1571987564.298,
						"629715"
					]
				}
			]
		}
	}

*/

import (
	"SLALite/assessment/monitor"
	"SLALite/assessment/monitor/genericadapter"
	"SLALite/model"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	// Name is the unique identifier of this adapter/retriever
	Name = "prometheus"

	// PrometheusURLPropertyName is the config property name of the Prometheus URL
	PrometheusURLPropertyName = "prometheusUrl"

	// defaultURL is the value of the Prometheus URL is PrometheusURLPropertyName is not set
	defaultURL = "http://localhost:9090"

	vectorType resultType = "vector"
	matrixType resultType = "matrix"
)

// Retriever implements genericadapter.Retrieve
type Retriever struct {
	URL string
}

// New constructs a Prometheus adapter from a Viper configuration
func New(config *viper.Viper) Retriever {

	config.SetDefault(PrometheusURLPropertyName, defaultURL)
	logConfig(config)

	return Retriever{
		config.GetString(PrometheusURLPropertyName),
	}
}

func logConfig(config *viper.Viper) {
	log.Infof("Prometheus configuration:\n"+
		"\tURL: %s", config.GetString(PrometheusURLPropertyName))
}

// Retrieve implements genericadapter.Retrieve
func (r Retriever) Retrieve() genericadapter.Retrieve {

	return func(agreement model.Agreement,
		items []monitor.RetrievalItem) map[model.Variable][]model.MetricValue {

		rootURL := r.prometheusRoot(agreement)
		result := make(map[model.Variable][]model.MetricValue)
		for _, item := range items {			
			url := fmt.Sprintf("%s/api/v1/query?query=%s",
				rootURL, item.Var.Metric)
			query := r.request(url)
			aux := translateVector(query, item.Var.Name)
			result[item.Var] = aux
		}
		return result
	}
}

func (r Retriever) prometheusRoot(agreement model.Agreement) string {
	if agreement.Assessment.MonitoringURL != "" {
		return agreement.Assessment.MonitoringURL
	}
	return r.URL
}

func (r Retriever) request(url string) query {

	resp, err := http.Get(url)
	if err != nil {
		log.Error(err)
		return query{}
	}
	defer resp.Body.Close()

	var result query
	if resp.Status[0] != '2' { // StatusCode < 200 || resp.StatusCode >= 300
		log.Errorf("%s GET %s", resp.Status, url)
	}
	log.Infof("%d %s", resp.StatusCode, url)
	err = parse(resp.Body, &result)
	if err != nil {
		log.Errorf("Error decoding prometheus output: %s", err.Error())
	}
	return result
}

func parse(r io.Reader, target *query) error {
	return json.NewDecoder(r).Decode(&target)
}

func translateVector(query query, key string) []model.MetricValue {

	res := make([]model.MetricValue, 0, len(query.Data.Results))
	for _, item := range query.Data.Results {
		metric := translateMetric(key, item)
		res = append(res, metric)
	}
	return res
}

// this function should be made project-dependent
func translateMetric(key string, item result) model.MetricValue {
	return model.MetricValue{
		Key:      fmt.Sprintf("%s{%s}", key, item.Metric.Instance),
		Value:    item.Item.Value,
		DateTime: time.Time(item.Item.Timestamp),
	}
}
