package runner

type RunConfig struct {
	Args []string
}

type Runner interface {
	Execute(run RunConfig)
}
