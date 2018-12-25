package graph

import (
	"errors"
	"fmt"
	config "github.com/justinbarrick/hone/pkg/job"
	"github.com/justinbarrick/hone/pkg/logger"
	"github.com/justinbarrick/hone/pkg/utils"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/topo"
	"sync"
)

type JobGraph struct {
	graph *simple.DirectedGraph
}

type Node struct {
	Job  *config.Job
	Done chan bool
}

func NewNode(job *config.Job) *Node {
	return &Node{
		Job:  job,
		Done: make(chan bool),
	}
}

func (n Node) ID() int64 {
	return n.Job.ID()
}

func NewJobGraph(jobs []*config.Job) (JobGraph, error) {
	graph := JobGraph{
		graph: simple.NewDirectedGraph(),
	}

	err := graph.BuildGraph(jobs)
	return graph, err
}

func (j *JobGraph) BuildGraph(jobs []*config.Job) error {
	jobsMap := map[string]*config.Job{}
	for _, job := range jobs {
		jobsMap[job.GetName()] = job
	}

	for _, job := range jobs {
		if j.graph.Node(job.ID()) == nil {
			j.graph.AddNode(NewNode(job))
		}

		if job.Deps != nil {
			for _, dep := range *job.Deps {
				depJob := jobsMap[dep]
				if depJob == nil {
					return errors.New(fmt.Sprintf("Dependency not found: %s", dep))
				}

				if j.graph.Node(depJob.ID()) == nil {
					j.graph.AddNode(NewNode(depJob))
				}

				j.graph.SetEdge(simple.Edge{
					T: j.graph.Node(job.ID()),
					F: j.graph.Node(depJob.ID()),
				})
			}
		}
	}

	return nil
}

func (j *JobGraph) WaitForDeps(n *Node, callback func(*config.Job) error, servicesWg *sync.WaitGroup) func(*config.Job) error {
	return func(job *config.Job) error {
		defer close(n.Done)

		failedDeps := []string{}

		for _, node := range graph.NodesOf(j.graph.To(n.ID())) {
			d := node.(*Node)
			_ = <-d.Done
			if d.Job.Error != nil {
				failedDeps = append(failedDeps, d.Job.GetName())
			}
		}

		if len(failedDeps) > 0 {
			job.Error = errors.New(fmt.Sprintf("Failed dependencies: %s", failedDeps))
			logger.LogError(job, job.Error.Error())
		}

		if job.Error != nil {
			return job.Error
		}

		if job.Service == true {
			servicesWg.Add(1)
			job.Detach = make(chan bool)

			go func() {
				defer close(job.Detach)
				defer servicesWg.Done()
				job.Error = callback(job)
			}()

			_ = <-job.Detach
		} else {
			job.Error = callback(job)
		}

		return job.Error
	}
}

func (j *JobGraph) IterTarget(target string, callback func(*Node) error) []error {
	targetId := utils.Crc(target)
	targetNode := j.graph.Node(targetId)
	if targetNode == nil {
		return []error{errors.New(fmt.Sprintf("Target %s not found.", target))}
	}

	sorted, err := topo.Sort(j.graph)
	if err != nil {
		return []error{err}
	}

	errors := []error{}
	for _, node := range sorted {
		if !topo.PathExistsIn(j.graph, node, targetNode) {
			continue
		}

		err := callback(node.(*Node))
		if err != nil {
			errors = append(errors, err)
		}
	}

	return errors
}

func (j *JobGraph) ResolveTarget(target string, callback func(*config.Job) error) []error {
	stopCh := make(chan bool)

	var wg sync.WaitGroup
	var servicesWg sync.WaitGroup

	errors := []error{}

	iterErrors := j.IterTarget(target, func(node *Node) error {
		wg.Add(1)

		go func(n *Node) {
			defer wg.Done()
			cb := j.WaitForDeps(n, callback, &servicesWg)
			n.Job.Stop = stopCh
			err := cb(n.Job)
			if err != nil {
				errors = append(errors, err)
			}
		}(node)

		return nil
	})

	errors = append(errors, iterErrors...)

	wg.Wait()
	close(stopCh)
	servicesWg.Wait()
	return errors
}

func (j *JobGraph) LongestTarget(target string) (int, []error) {
	longestJob := 0
	lock := sync.Mutex{}

	errors := j.IterTarget(target, func(n *Node) error {
		lock.Lock()

		name := n.Job.GetName()

		if len(name) > longestJob {
			longestJob = len(name)
		}

		lock.Unlock()
		return nil
	})

	return longestJob, errors
}
