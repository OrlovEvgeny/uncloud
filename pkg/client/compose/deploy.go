package compose

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compose-spec/compose-go/v2/graph"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/docker/api/types/volume"
	"github.com/psviderski/uncloud/pkg/api"
	"github.com/psviderski/uncloud/pkg/client/deploy"
	"github.com/psviderski/uncloud/pkg/client/deploy/scheduler"
)

type Client interface {
	api.DNSClient
	deploy.Client
}

type Deployment struct {
	Client       Client
	Project      *types.Project
	SpecResolver *deploy.ServiceSpecResolver
	state        *scheduler.ClusterState
	plan         *deploy.SequenceOperation
}

func NewDeployment(ctx context.Context, cli Client, project *types.Project) (*Deployment, error) {
	state, err := scheduler.InspectClusterState(ctx, cli)
	if err != nil {
		return nil, fmt.Errorf("inspect cluster state: %w", err)
	}

	domain, err := cli.GetDomain(ctx)
	if err != nil && !errors.Is(err, api.ErrNotFound) {
		return nil, fmt.Errorf("get cluster domain: %w", err)
	}

	resolver := &deploy.ServiceSpecResolver{
		// If the domain is not found (not reserved), an empty domain is used for the resolver.
		ClusterDomain: domain,
	}

	return &Deployment{
		Client:       cli,
		Project:      project,
		SpecResolver: resolver,
		state:        state,
	}, nil
}

func (d *Deployment) Plan(ctx context.Context) (deploy.SequenceOperation, error) {
	if d.plan != nil {
		return *d.plan, nil
	}
	plan := deploy.SequenceOperation{}

	// Generate service specs for all services in the project.
	var serviceSpecs []api.ServiceSpec
	err := graph.InDependencyOrder(ctx, d.Project,
		func(ctx context.Context, name string, _ types.ServiceConfig) error {
			spec, err := d.ServiceSpec(name)
			if err != nil {
				return err
			}

			serviceSpecs = append(serviceSpecs, spec)
			return nil
		})
	if err != nil {
		return plan, err
	}

	// Check external volumes and plan the creation of missing volumes before deploying services.
	volumeOps, err := d.planVolumes(serviceSpecs)
	if err != nil {
		return plan, err
	}
	for _, op := range volumeOps {
		plan.Operations = append(plan.Operations, op)
	}

	for _, spec := range serviceSpecs {
		// TODO: properly handle depends_on conditions in the service deployment plan as the first operation.
		// Pass the update cluster state with scheduled volumes to the deployment.
		deployment := deploy.NewDeployment(d.Client, spec, &deploy.RollingStrategy{State: d.state})
		servicePlan, err := deployment.Plan(ctx)
		if err != nil {
			return plan, fmt.Errorf("create deployment plan for service '%s': %w", spec.Name, err)
		}

		// Skip no-op (up-to-date) service plans.
		if len(servicePlan.Operations) > 0 {
			plan.Operations = append(plan.Operations, &servicePlan)
		}
	}

	d.plan = &plan
	return plan, nil
}

// ServiceSpec returns the service specification for the given compose service that is ready for deployment.
func (d *Deployment) ServiceSpec(name string) (api.ServiceSpec, error) {
	spec, err := ServiceSpecFromCompose(d.Project, name)
	if err != nil {
		return spec, fmt.Errorf("convert compose service '%s' to service spec: %w", name, err)
	}

	return spec, nil
}

// PlanVolumes checks if the external volumes exist and plans the creation of missing volumes.
func (d *Deployment) planVolumes(serviceSpecs []api.ServiceSpec) ([]*deploy.CreateVolumeOperation, error) {
	if len(d.Project.Volumes) == 0 {
		// No volumes to check or create.
		return nil, nil
	}

	if err := d.checkExternalVolumesExist(); err != nil {
		return nil, err
	}

	// TODO: The scheduler should ideally work with the resolved service specs to correctly identify eligible machines.
	//  Figure out where the best place to resolve the specs is.
	volumeScheduler, err := scheduler.NewVolumeScheduler(d.state, serviceSpecs)
	if err != nil {
		return nil, fmt.Errorf("init volume scheduler: %w", err)
	}
	scheduledVolumes, err := volumeScheduler.Schedule()
	if err != nil {
		return nil, fmt.Errorf("schedule volumes: %w", err)
	}

	// Generate operations to create scheduled missing volumes.
	var ops []*deploy.CreateVolumeOperation
	for machineID, volumes := range scheduledVolumes {
		for _, v := range volumes {
			ops = append(ops, &deploy.CreateVolumeOperation{
				MachineID:  machineID,
				VolumeSpec: v,
			})
		}
	}

	return ops, nil
}

// checkExternalVolumesExist checks that all external volumes exist in the cluster.
func (d *Deployment) checkExternalVolumesExist() error {
	var externalNames []string
	for _, v := range d.Project.Volumes {
		if v.External {
			externalNames = append(externalNames, v.Name)
		}
	}

	var notFound []string
	for _, name := range externalNames {
		if !slices.ContainsFunc(d.state.Machines, func(m *scheduler.Machine) bool {
			return slices.ContainsFunc(m.Volumes, func(vol volume.Volume) bool {
				return vol.Name == name
			})
		}) {
			notFound = append(notFound, fmt.Sprintf("'%s'", name))
		}
	}

	if len(notFound) > 0 {
		return fmt.Errorf("external volumes not found: %s", strings.Join(notFound, ", "))
	}
	return nil
}

func (d *Deployment) Run(ctx context.Context) error {
	plan, err := d.Plan(ctx)
	if err != nil {
		return fmt.Errorf("create plan: %w", err)
	}

	return plan.Execute(ctx, d.Client)
}
