package rest

type PipelineParameter struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	IsFlag bool   `json:"is_flag"`
}

type Executor struct {
	Name            string `json:"name"`
	ComputeOverride string `json:"compute_override"`
}

type RunRequest struct {
	PipelineUrl string              `json:"pipeline_url"`
	Executor    Executor            `json:"executor"`
	Parameters  []PipelineParameter `json:"parameters"`
}

type RunResponse struct {
	Status     bool   `json:"status"`
	ProcessKey string `json:"process_key"`
	Executor   string `json:"executor"`
}

func (p PipelineParameter) String() []string {
	if p.IsFlag {
		return []string{p.Key}
	}

	return []string{p.Key, p.Value}
}

func (r RunRequest) Args() []string {
	args := make([]string, 0, len(r.Parameters))
	for _, p := range r.Parameters {
		args = append(args, p.String()...)
	}

	return args
}

type StopJobRequest struct {
	ProcessId string `json:"process_id"`
	Executor  string `json:"executor"`
}

type StopJobResponse struct {
	Status bool `json:"status"`
}
