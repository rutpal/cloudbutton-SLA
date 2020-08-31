/*
Copyright 2018 Atos

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

// Package rabbitnotifier contains a simple ViolationsNotifier that send violations to a rabbit queue.
package rabbitnotifier

import (
	assessment_model "SLALite/assessment/model"
	"SLALite/model"
	"encoding/json"

	log "github.com/sirupsen/logrus"
	"github.com/streadway/amqp"
)

const (
	// Name is the unique identifier of this notifier
	Name = "rabbit"
)

//RabbitNotifier logs violations
type RabbitNotifier struct {
}

var q amqp.Queue
var ch *amqp.Channel
var conn *amqp.Connection

//Error traitment
func failOnError(err error, msg string) {
	if err != nil {
		log.Error("%s: %s", msg, err)
	}
}

//ConnectQueue connect to Rabbit queue
func ConnectQueue() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")

	ch, err = conn.Channel()
	failOnError(err, "Failed to open a channel")

	q, err = ch.QueueDeclare(
		"Cloudbutton", // name
		true,          // durable
		false,         // delete when unused
		false,         // exclusive
		false,         // no-wait
		nil,           // arguments
	)

	failOnError(err, "Failed to declare a queue")
}

// NotifyViolations implements ViolationNotifier interface
func (n RabbitNotifier) NotifyViolations(agreement *model.Agreement, result *assessment_model.Result) {
	ConnectQueue()
	body := make(map[string]interface{})
	fields := make(map[string]interface{})
	log.Info("Violation of agreement: " + agreement.Id)
	for k, v := range result.Violated {
		if len(v.Violations) > 0 {
			log.Info("Failed guarantee: " + k)
			for _, vi := range v.Violations {
				log.Infof("Failed guarantee %v of agreement %s at %s", vi.Guarantee, vi.AgreementId, vi.Datetime)
				fields["IdAgreement"] = vi.AgreementId
				fields["Guarantee"] = vi.Guarantee
				fields["ViolationTime"] = vi.Datetime
				body["Application"] = vi.AgreementId
				body["Message"] = "QoS_Violation"
				body["Fields"] = fields

				jsonData, err := json.Marshal(body)
				//				log.Info(jsonData)
				failOnError(err, "Failed to Marshal body")

				err = ch.Publish(
					"",     // exchange
					q.Name, // routing key
					false,  // mandatory
					false,  // immediate
					amqp.Publishing{
						ContentType: "application/json",
						Body:        jsonData,
					})
				failOnError(err, "Failed to publish a message")
				log.Infof("Violation Message Published on queue %s", q.Name)
			}
		}
	}
}
