package job

import (
	"errors"
	"fmt"
	"github.com/justinbarrick/hone/pkg/utils"
	"strings"
)

type Job struct {
	Name    string             `hcl:"name,label"`
	Template *string `hcl:"template" hash:"-"`
	Image   *string            `hcl:"image"`
	Shell   *string            `hcl:"shell"`
	Exec    *[]string          `hcl:"exec"`
	Inputs  *[]string          `hcl:"inputs"`
	Input   *string            `hcl:"input"`
	Outputs *[]string          `hcl:"outputs"`
	Output  *string            `hcl:"output"`
	Env     *map[string]string `hcl:"env"`
	Deps    *[]string          `hcl:"deps"`
	Engine  *string            `hcl:"engine" hash:"-"`
	Condition *string          `hcl:"condition"`
	Error   error              `hash:"-"`
}

func (j *Job) Default(def Job) {
	if j.Image == nil {
		j.Image = def.Image
	}

	if j.Shell == nil {
		j.Shell = def.Shell
	}

	if j.Exec == nil {
		j.Exec = def.Exec
	}

	if j.Inputs == nil && j.Input == nil {
		j.Inputs = def.Inputs
		j.Input = def.Input
	}

	if j.Outputs == nil && j.Output == nil {
		j.Outputs = def.Outputs
		j.Output = def.Output
	}

	if j.Engine == nil {
		j.Engine = def.Engine
	}

	if j.Deps == nil {
		j.Deps = def.Deps
	}

	if def.Env != nil {
		if j.Env == nil {
			j.Env = def.Env
		} else {
			env := *j.Env

			for key, value := range *def.Env {
				if env[key] != "" {
					continue
				}

				env[key] = value
			}

			j.Env = &env
		}
	}
}

func (j Job) Validate(engine string) error {
	myEngine := j.GetEngine()
	if myEngine == "" {
		myEngine = engine
	}

	if j.Image == nil && myEngine != "local" {
		return errors.New("Image is required when engine is not local.")
	}

	if j.Shell != nil && j.Exec != nil {
		return errors.New("Shell and exec are mutually exclusive.")
	}

	return nil
}

func (j Job) ID() int64 {
	return utils.Crc(j.GetName())
}

func (j Job) GetName() string {
	return j.Name
}

func (j Job) GetImage() string {
	image := *j.Image

	if !strings.Contains(image, ":") {
		image = fmt.Sprintf("%s:latest", image)
	}

	return image
}

func (j Job) GetOutputs() []string {
	outputs := []string{}

	if j.Outputs != nil {
		outputs = *j.Outputs
	}

	if j.Output != nil {
		outputs = append(outputs, *j.Output)
	}

	return outputs
}

func (j Job) GetInputs() []string {
	inputs := []string{}

	if j.Inputs != nil {
		inputs = *j.Inputs
	}

	if j.Input != nil {
		inputs = append(inputs, *j.Input)
	}

	return inputs
}

func (j Job) GetShell() []string {
	if j.Exec != nil {
		return *j.Exec
	} else {
		return []string{
			"/bin/sh", "-cex", *j.Shell,
		}
	}
}

func (j Job) GetEngine() string {
	if j.Engine != nil {
		return *j.Engine
	} else {
		return ""
	}
}

func (j Job) GetEnv() map[string]string {
	if j.Env == nil {
		return map[string]string{}
	}
	return *j.Env
}
