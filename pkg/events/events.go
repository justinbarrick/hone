package events

import (
	"fmt"
	"github.com/justinbarrick/hone/pkg/job"
	"github.com/justinbarrick/hone/pkg/logger"
	"github.com/caibirdme/yql"
)

func YQLMatch(condition *string, env map[string]interface{}) (bool, error) {
	if condition == nil {
		return true, nil
	}

	return yql.Match(*condition, env)
}

func EventCallback(env map[string]interface{}, cb func (j *job.Job) error) (func (j *job.Job) error) {
	return func (j *job.Job) error {
		run, err := YQLMatch(j.Condition, env)
		if err != nil {
			return err
		}
		
		if run {
			return cb(j)
		}

		logger.Log(j, fmt.Sprintf("Skipping job, since condition not met: %s", *j.Condition))
		return nil
	}
}
