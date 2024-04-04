package plugin

const (
	ConstListInstancesFilter = "filter"
	ConstInstanceGroup       = "instance_group"
)

var allowedSetFields = map[string]struct{}{
	ConstListInstancesFilter: {},
	ConstInstanceGroup:       {},
}
