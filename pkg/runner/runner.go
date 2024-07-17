package runner

type RunConfig struct {
	PipelineUrl    string
	ConfigOverride string
	Args           []string
}

type Runner interface {
	Execute(run RunConfig)
}

func (r RunConfig) CmdArgs() []string {
	return append([]string{"run", r.PipelineUrl}, r.Args...)
}
