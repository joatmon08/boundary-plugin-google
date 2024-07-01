package plugin

import (
	"context"
	"fmt"

	"github.com/hashicorp/boundary/sdk/pbs/controller/api/resources/hostsets"
	pb "github.com/hashicorp/boundary/sdk/pbs/plugin"
	errors "github.com/chpag/boundary-plugin-google/internal/errors"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GooglePlugin struct {
	pb.UnimplementedHostPluginServiceServer
}

var (
	_ pb.HostPluginServiceServer = (*GooglePlugin)(nil)
)

func (p *GooglePlugin) OnCreateCatalog(_ context.Context, req *pb.OnCreateCatalogRequest) (*pb.OnCreateCatalogResponse, error) {
	catalog := req.GetCatalog()
	if catalog == nil {
		return nil, status.Error(codes.InvalidArgument, "catalog is nil")
	}

	attrs := catalog.GetAttributes()
	if attrs == nil {
		return nil, status.Error(codes.InvalidArgument, "attributes are required")
	}

	if _, err := getCatalogAttributes(attrs); err != nil {
		return nil, err
	}

	return &pb.OnCreateCatalogResponse{
		Persisted: &pb.HostCatalogPersisted{
			Secrets: nil,
		},
	}, nil
}

func (p *GooglePlugin) OnUpdateCatalog(_ context.Context, req *pb.OnUpdateCatalogRequest) (*pb.OnUpdateCatalogResponse, error) {
	currentCatalog := req.GetCurrentCatalog()
	if currentCatalog == nil {
		return nil, status.Error(codes.FailedPrecondition, "current catalog is nil")
	}

	secrets := req.GetNewCatalog().GetSecrets()
	if secrets == nil {
		// If new secrets weren't passed in, don't rotate what we have on
		// update.
		return &pb.OnUpdateCatalogResponse{}, nil
	}

	return &pb.OnUpdateCatalogResponse{
		Persisted: &pb.HostCatalogPersisted{
			Secrets: nil,
		},
	}, nil
}

func (p *GooglePlugin) OnDeleteCatalog(ctx context.Context, req *pb.OnDeleteCatalogRequest) (*pb.OnDeleteCatalogResponse, error) {
	catalog := req.GetCatalog()
	if catalog == nil {
		return nil, status.Error(codes.InvalidArgument, "new catalog is nil")
	}

	attrs := catalog.GetAttributes()
	if attrs == nil {
		return nil, status.Error(codes.InvalidArgument, "new catalog missing attributes")
	}

	if _, err := getCatalogAttributes(attrs); err != nil {
		return nil, err
	}

	return &pb.OnDeleteCatalogResponse{}, nil
}

func (p *GooglePlugin) OnCreateSet(_ context.Context, req *pb.OnCreateSetRequest) (*pb.OnCreateSetResponse, error) {
	if err := validateSet(req.GetSet()); err != nil {
		return nil, err
	}
	return &pb.OnCreateSetResponse{}, nil
}

func (p *GooglePlugin) OnUpdateSet(_ context.Context, req *pb.OnUpdateSetRequest) (*pb.OnUpdateSetResponse, error) {
	if err := validateSet(req.GetNewSet()); err != nil {
		return nil, err
	}
	return &pb.OnUpdateSetResponse{}, nil
}

// OnDeleteSet is called when a dynamic host set is deleted.
func (p *GooglePlugin) OnDeleteSet(ctx context.Context, req *pb.OnDeleteSetRequest) (*pb.OnDeleteSetResponse, error) {
	return &pb.OnDeleteSetResponse{}, nil
}

func (p *GooglePlugin) ListHosts(ctx context.Context, req *pb.ListHostsRequest) (*pb.ListHostsResponse, error) {
	catalog := req.GetCatalog()
	if catalog == nil {
		return nil, status.Error(codes.InvalidArgument, "catalog is nil")
	}

	catalogAttrsRaw := catalog.GetAttributes()
	if catalogAttrsRaw == nil {
		return nil, status.Error(codes.InvalidArgument, "catalog missing attributes")
	}

	catalogAttributes, err := getCatalogAttributes(catalogAttrsRaw)
	if err != nil {
		return nil, err
	}

	sets := req.GetSets()
	if sets == nil {
		return nil, status.Error(codes.InvalidArgument, "sets is nil")
	}

	hosts := []*pb.ListHostsResponseHost{}
	for _, set := range sets {
		if set.GetId() == "" {
			return nil, status.Error(codes.InvalidArgument, "set missing id")
		}

		if set.GetAttributes() == nil {
			return nil, status.Error(codes.InvalidArgument, "set missing attributes")
		}
		setAttrs, err := getSetAttributes(set.GetAttributes())
		if err != nil {
			return nil, err
		}

		if setAttrs.InstanceGroup != "" {
			hosts, err = getInstancesForInstanceGroup(ctx, set.GetId(), setAttrs, catalogAttributes)
			if err != nil {
				return nil, err
			}
		} else {
			request := buildListInstancesRequest(setAttrs, catalogAttributes)
			hosts, err = getInstances(ctx, set.GetId(), request)
			if err != nil {
				return nil, err
			}
		}
	}

	return &pb.ListHostsResponse{
		Hosts: hosts,
	}, nil
}

func validateSet(s *hostsets.HostSet) error {
	if s == nil {
		return status.Error(codes.InvalidArgument, "set is nil")
	}
	var attrs SetAttributes
	attrMap := s.GetAttributes().AsMap()
	if err := mapstructure.Decode(attrMap, &attrs); err != nil {
		return status.Errorf(codes.InvalidArgument, "error decoding set attributes: %s", err)
	}

	badFields := make(map[string]string)
	_, filterSet := attrMap[ConstListInstancesFilter]
	_, instanceGroupSet := attrMap[ConstInstanceGroup]

	if instanceGroupSet && filterSet {
		badFields["attributes"] = "must set instance group or filter, cannot set both"
	} else if instanceGroupSet && len(attrs.InstanceGroup) == 0 {
		badFields[fmt.Sprintf("attributes.%s", ConstInstanceGroup)] = "must not be empty."
	} else if filterSet && len(attrs.Filter) == 0 {
		badFields[fmt.Sprintf("attributes.%s", ConstListInstancesFilter)] = "must not be empty."
	}

	for f := range attrMap {
		if _, ok := allowedSetFields[f]; !ok {
			badFields[fmt.Sprintf("attributes.%s", f)] = "Unrecognized field."
		}
	}

	if len(badFields) > 0 {
		return errors.InvalidArgumentError("Invalid arguments in the new set", badFields)
	}
	return nil
}
