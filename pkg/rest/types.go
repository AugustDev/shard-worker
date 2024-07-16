package rest

type PipelineParameter struct {
	Key             string `json:"name"`
	Value           string `json:"value"`
	IsFlag          bool   `json:"isFlag"`
	ComputeOverride string `json:"compute_override"`
}

type Executor struct {
	Name string `json:"name"`
}

type RunRequest struct {
	PipelineUrl string              `json:"pipeline_url"`
	Executor    Executor            `json:"executor"`
	Parameters  []PipelineParameter `json:"parameters"`
}

type RunResponse struct {
	Status bool `json:"status"`
}
