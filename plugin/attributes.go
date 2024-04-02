package plugin

import (
	"fmt"

	computepb "cloud.google.com/go/compute/apiv1/computepb"
	cred "github.com/joatmon08/boundary-plugin-google/internal/credential"
	"github.com/joatmon08/boundary-plugin-google/internal/errors"
	"github.com/joatmon08/boundary-plugin-google/internal/values"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

type CatalogAttributes struct {
	*cred.CredentialAttributes
}

func getCatalogAttributes(in *structpb.Struct) (*CatalogAttributes, error) {
	unknownFields := values.StructFields(in)
	badFields := make(map[string]string)

	var err error
	credAttributes, err := cred.GetCredentialAttributes(in)
	if err != nil {
		return nil, err
	}

	for s := range unknownFields {
		switch s {
		// Ignore knownFields from CredentialAttributes
		case cred.ConstProject:
			continue
		case cred.ConstZone:
			continue
		case cred.ConstInstanceGroup:
			continue
		default:
			badFields[fmt.Sprintf("attributes.%s", s)] = "unrecognized field"
		}
	}

	if len(badFields) > 0 {
		return nil, errors.InvalidArgumentError("Invalid arguments in catalog attributes", badFields)
	}

	return &CatalogAttributes{
		CredentialAttributes: credAttributes,
	}, nil
}

type SetAttributes struct {
	Filter string
}

func getSetAttributes(in *structpb.Struct) (*SetAttributes, error) {
	var setAttrs SetAttributes

	unknownFields := values.StructFields(in)
	badFields := make(map[string]string)
	delete(unknownFields, ConstListInstancesFilter)
	for a := range unknownFields {
		badFields[fmt.Sprintf("attributes.%s", a)] = "unrecognized field"
	}
	if len(badFields) > 0 {
		return nil, errors.InvalidArgumentError("Error in the attributes provided", badFields)
	}

	// Mapstructure complains if it expects a slice as output and sees a scalar
	// value. Rather than use WeakDecode and risk unintended consequences, I'm
	// manually making this change if necessary.
	inMap := in.AsMap()
	if filtersRaw, ok := inMap[ConstListInstancesFilter]; ok {
		switch filterVal := filtersRaw.(type) {
		case string:
			inMap[ConstListInstancesFilter] = string(filterVal)
		}
	}

	if err := mapstructure.Decode(inMap, &setAttrs); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "error decoding set attributes: %s", err)
	}

	return &setAttrs, nil
}

func buildListInstancesRequest(attributes *SetAttributes, catalog *CatalogAttributes) *computepb.ListInstancesRequest {
	instanceRequest := &computepb.ListInstancesRequest{
		Project: catalog.Project,
		Zone:    catalog.Zone,
	}

	if len(attributes.Filter) > 1 {
		instanceRequest.Filter = &attributes.Filter
	}

	return instanceRequest
}
