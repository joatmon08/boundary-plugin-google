package plugin

import (
	"context"
	"fmt"

	compute "cloud.google.com/go/compute/apiv1"
	"github.com/hashicorp/boundary/sdk/pbs/controller/api/resources/hostsets"
	pb "github.com/hashicorp/boundary/sdk/pbs/plugin"
	errors "github.com/joatmon08/boundary-plugin-google/internal/errors"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/api/iterator"
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

		request := buildListInstancesRequest(setAttrs, catalogAttributes)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "error building ListInstancesRequest parameters for host set id %q: %s", set.GetId(), err)
		}

		c, err := compute.NewInstancesRESTClient(ctx)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "error creating NewInstancesRESTClient for host set id %q: %s", set.GetId(), err)
		}

		it := c.List(ctx, request)
		for {
			resp, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "error listing instances for host set id %q: %s", set.GetId(), err)
			}
			host, err := instanceToHost(resp)
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "error processing host results for host set id %q: %s", set.GetId(), err)
			}
			hosts = append(hosts, host)
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
	if _, ok := attrMap[ConstListInstancesFilter]; ok && len(attrs.Filter) == 0 {
		badFields[fmt.Sprintf("attributes.%s", ConstListInstancesFilter)] = "This field must be not empty."
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
