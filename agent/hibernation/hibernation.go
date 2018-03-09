// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package hibernation is responsible for the agent in hibernate mode.
// It depends on health pings in an exponential backoff to check if the agent needs
// to move to active mode.
package hibernation

import (
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/health"
	"github.com/carlescere/scheduler"
)

// Hibernate holds information about the current agent state
type Hibernate struct {
	currentMode  health.AgentState
	healthModule *health.HealthCheck
	context      context.T
	//TODO: Add backoff object here. Will scheduler still be needed?
	hibernateJob *scheduler.Job

	scheduleHealthPing func(pingInterval int, m *Hibernate)
}

// modeChan is a channel that tracks the status of the agent
var modeChan = make(chan health.AgentState, 10)
var pingInterval = 5 * 60 // TODO Change when backoff is implemented

const (
	hibernateMode  = "AgentHibernate"
	backoffSeconds = 5 //TODO change once backoff is implemented
)

// NewHibernateMode creates an object of type NewHibernateMode
func NewHibernateMode(healthModule *health.HealthCheck, context context.T) *Hibernate {

	hibernationContext := context.With("[" + hibernateMode + "]")

	return &Hibernate{
		healthModule:       healthModule,
		currentMode:        health.Passive,
		context:            hibernationContext,
		scheduleHealthPing: scheduleEmptyHealthPing,
	}
}

// ExecuteHibernation Starts the hibernate mode by blocking agent start and by scheduling health pings
func ExecuteHibernation(m *Hibernate) health.AgentState {
	next := time.Duration(backoffSeconds) * time.Second
	// Wait backoff time and then schedule health pings
	<-time.After(next)
	m.scheduleHealthPing(pingInterval, m)

loop:
	// using an infinite loop to block the agent from starting
	for {
		// block and wait for health mode to be active
		status := <-modeChan
		switch status {
		case health.Active:
			//Agent mode is now active. Agent can start. Exit loop
			m.stopEmptyPing()
			return status //returning status for testing purposes.
		case health.Passive:
			continue loop
		default:
			continue loop
		}
	}
}

func (m *Hibernate) healthCheck() {
	modeChan <- health.GetAgentState(m.healthModule)
}

func scheduleEmptyHealthPing(pingInterval int, m *Hibernate) {
	var err error
	if m.hibernateJob, err = scheduler.Every(pingInterval).Seconds().Run(m.healthCheck); err != nil {
		m.context.Log().Errorf("unable to schedule health update. %v", err)
	}
	return
}

func (m *Hibernate) stopEmptyPing() {
	//TODO: Must this be called from elsewhere as well? where?
	if m.hibernateJob != nil {
		m.hibernateJob.Quit <- true
	}
}