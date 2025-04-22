package scheduler

import (
	"reflect"
	"slices"
	"strings"

	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	"github.com/psviderski/uncloud/pkg/api"
)

// Constraint is the base interface for all scheduling constraints.
type Constraint interface {
	// Evaluate determines if a machine satisfies the constraint.
	Evaluate(machine *Machine) bool

	// Description returns a human-readable description of the constraint.
	Description() string
}

// constraintsFromSpec derives scheduling constraints from the service specification.
func constraintsFromSpec(spec api.ServiceSpec) []Constraint {
	var constraints []Constraint

	// TODO: add placement constraint based on the supported platforms of the image.
	// TODO: add placement constraint to limit machines with the image if pull policy is never.

	if len(spec.Placement.Machines) > 0 {
		constraints = append(constraints, &PlacementConstraint{
			Machines: spec.Placement.Machines,
		})
	}

	// Add a VolumesConstraint for named Docker volumes that are mounted in the container.
	var volumes []api.VolumeSpec
	for _, m := range spec.Container.VolumeMounts {
		if v, ok := spec.Volume(m.VolumeName); ok && v.Type == api.VolumeTypeVolume {
			volumes = append(volumes, v)
		}
	}
	if len(volumes) > 0 {
		constraints = append(constraints, &VolumesConstraint{
			Volumes: volumes,
		})
	}

	return constraints
}

type PlacementConstraint struct {
	// Machines is a list of machine names or IDs where service containers are allowed to be deployed.
	// If empty, containers can be deployed to any available machine in the cluster.
	Machines []string
}

func (c *PlacementConstraint) Evaluate(machine *Machine) bool {
	for _, nameOrID := range c.Machines {
		if machine.Info.Id == nameOrID || machine.Info.Name == nameOrID {
			return true
		}
	}
	return false
}

func (c *PlacementConstraint) Description() string {
	slices.Sort(c.Machines)
	return "Placement constraint by machines: " + strings.Join(c.Machines, ", ")
}

// VolumesConstraint restricts container placement to machines that have the required named Docker volumes.
type VolumesConstraint struct {
	// Volumes is a list of named Docker volumes of type api.VolumeTypeVolume that must exist on the machine.
	Volumes []api.VolumeSpec
}

// Evaluate determines if a machine has all the required volumes.
// Returns true if all required volumes exist or scheduled on the machine or if there are no required volumes.
func (c *VolumesConstraint) Evaluate(machine *Machine) bool {
	if len(c.Volumes) == 0 {
		return true
	}

	for _, v := range c.Volumes {
		if v.Type != api.VolumeTypeVolume {
			continue
		}

		// Check if the required volume already exists on the machine.
		if slices.ContainsFunc(machine.Volumes, func(vol volume.Volume) bool {
			if v.DockerVolumeName() == vol.Name {
				return v.MatchesDockerVolume(vol)
			}
			return false
		}) {
			continue
		}

		// Check if the required volume has been scheduled on the machine. The driver names and options must match.
		if !slices.ContainsFunc(machine.ScheduledVolumes, func(scheduled api.VolumeSpec) bool {
			if v.DockerVolumeName() != scheduled.DockerVolumeName() {
				return false
			}

			// The volume spec with an empty driver can mount the volume that matches the name no matter the driver.
			if v.VolumeOptions.Driver == nil {
				return true
			}

			// If the driver is specified in the spec, the spec's driver and options must match the volume's driver
			// and options to successfully mount the volume.
			scheduled = scheduled.SetDefaults()
			scheduledDriver := scheduled.VolumeOptions.Driver
			if scheduledDriver == nil {
				scheduledDriver = &mount.Driver{Name: api.VolumeDriverLocal}
			}
			return reflect.DeepEqual(v.VolumeOptions.Driver, scheduledDriver)
		}) {
			return false
		}
	}

	return true
}

func (c *VolumesConstraint) Description() string {
	volumeNames := make([]string, 0, len(c.Volumes))
	for _, v := range c.Volumes {
		if v.Type == api.VolumeTypeVolume {
			volumeNames = append(volumeNames, v.DockerVolumeName())
		}
	}
	slices.Sort(volumeNames)

	if len(volumeNames) == 0 {
		return "No volumes constraint"
	}

	return "Volumes: " + strings.Join(volumeNames, ", ")
}
