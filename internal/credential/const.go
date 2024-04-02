package credential

const (
	ConstProject       = "project"
	ConstZone          = "zone"
	ConstInstanceGroup = "instance_group"
)

var AllowedCatalogFields = map[string]struct{}{
	ConstProject:       {},
	ConstZone:          {},
	ConstInstanceGroup: {},
}
