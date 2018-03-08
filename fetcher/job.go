package fetcher

import (
	"context"
)

var jobIndex = make(map[string]func() Job)

// A Job is a job enqueued to the Fetcher topic via a FetchMessage.
type Job interface {
	// Type returns a type identifying this type of job, eg. "character" or "free_company".
	Type() string

	// Run runs the job, and returns any resulting jobs to be enqueued or an error.
	Run(context.Context) ([]Job, error)
}

func registerJob(fn func() Job) {
	jobIndex[fn().Type()] = fn
}
