package model

func (p Parameter) String() []string {
	if p.IsFlag {
		return []string{p.Key}
	}

	return []string{p.Key, p.Value}
}

func (r RunJobCommand) Args() []string {
	args := make([]string, 0, len(r.Parameters))
	for _, p := range r.Parameters {
		args = append(args, p.String()...)
	}

	return args
}
