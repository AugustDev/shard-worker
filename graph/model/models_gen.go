// Code generated by github.com/99designs/gqlgen, DO NOT EDIT.

package model

type Executor struct {
	Name            string `json:"name"`
	ComputeOverride string `json:"computeOverride"`
}

type Log struct {
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

type Mutation struct {
}

type Parameter struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	IsFlag bool   `json:"isFlag"`
}

type Query struct {
}

type RunJobCommand struct {
	RunName     string       `json:"runName"`
	PipelineURL string       `json:"pipelineUrl"`
	Executor    *Executor    `json:"executor"`
	Parameters  []*Parameter `json:"parameters"`
}

type RunJobResponse struct {
	Status     bool   `json:"status"`
	ProcessKey string `json:"processKey"`
	Executor   string `json:"executor"`
	RunName    string `json:"runName"`
}

type Subscription struct {
}

type TerminateJobCommand struct {
	ProcessKey string `json:"processKey"`
	Executor   string `json:"executor"`
}
