package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/boundary/sdk/pbs/controller/api/resources/hostcatalogs"
	"github.com/hashicorp/boundary/sdk/pbs/controller/api/resources/hostsets"
	"github.com/hashicorp/boundary/sdk/pbs/plugin"
	pb "github.com/hashicorp/boundary/sdk/pbs/plugin"
	"github.com/hashicorp/go-secure-stdlib/parseutil"
	cred "github.com/joatmon08/boundary-plugin-google/internal/credential"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func wrapMap(t *testing.T, in map[string]interface{}) *structpb.Struct {
	t.Helper()
	out, err := structpb.NewStruct(in)
	require.NoError(t, err)
	return out
}

func TestListHosts(t *testing.T) {
	ctx := context.Background()
	p := &GooglePlugin{}

	wd, err := os.Getwd()
	require.NoError(t, err)
	require.NotEmpty(t, wd)
	project, err := parseutil.ParsePath("file://" + filepath.Join(wd, "secrets", "project"))
	require.NoError(t, err)
	zone, err := parseutil.ParsePath("file://" + filepath.Join(wd, "secrets", "zone"))
	require.NoError(t, err)

	hostCatalogAttributes := &hostcatalogs.HostCatalog_Attributes{
		Attributes: wrapMap(t, map[string]interface{}{
			cred.ConstProject: project,
			cred.ConstZone:    zone,
		}),
	}

	cases := []struct {
		name        string
		req         *pb.ListHostsRequest
		expected    []*pb.ListHostsResponseHost
		expectedErr string
	}{
		{
			name:        "nil catalog",
			req:         &pb.ListHostsRequest{},
			expectedErr: "catalog is nil",
		},
		{
			name: "project not defined",
			req: &pb.ListHostsRequest{
				Catalog: &hostcatalogs.HostCatalog{
					Attrs: &hostcatalogs.HostCatalog_Attributes{
						Attributes: wrapMap(t, map[string]interface{}{
							cred.ConstZone: zone,
						}),
					},
				},
			},
			expectedErr: "attributes.project: missing required value \"project\"",
		},
		{
			name: "get all three instances",
			req: &pb.ListHostsRequest{
				Catalog: &hostcatalogs.HostCatalog{
					Attrs: hostCatalogAttributes,
				},
				Sets: []*hostsets.HostSet{
					{
						Id: "get-all-instances",
						Attrs: &hostsets.HostSet_Attributes{
							Attributes: wrapMap(t, map[string]interface{}{}),
						},
					},
				},
			},
			expected: []*pb.ListHostsResponseHost{
				{
					Name: "boundary-0",
				},
				{
					Name: "boundary-1",
				},
				{
					Name: "boundary-2",
				},
			},
		},
		{
			name: "get one instance",
			req: &pb.ListHostsRequest{
				Catalog: &hostcatalogs.HostCatalog{
					Attrs: hostCatalogAttributes,
				},
				Sets: []*hostsets.HostSet{
					{
						Id: "get-all-instances",
						Attrs: &hostsets.HostSet_Attributes{
							Attributes: wrapMap(t, map[string]interface{}{
								ConstListInstancesFilter: "name = boundary-1",
							}),
						},
					},
				},
			},
			expected: []*pb.ListHostsResponseHost{
				{
					Name: "boundary-1",
				},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require := require.New(t)

			actual, err := p.ListHosts(ctx, tc.req)
			if tc.expectedErr != "" {
				require.Contains(err.Error(), tc.expectedErr)
				return
			}

			require.NoError(err)
			require.Equal(len(tc.expected), len(actual.GetHosts()))
		})
	}
}

func TestCreateCatalog(t *testing.T) {
	ctx := context.Background()
	p := &GooglePlugin{}

	wd, err := os.Getwd()
	require.NoError(t, err)
	require.NotEmpty(t, wd)
	project, err := parseutil.ParsePath("file://" + filepath.Join(wd, "secrets", "project"))
	require.NoError(t, err)
	zone, err := parseutil.ParsePath("file://" + filepath.Join(wd, "secrets", "zone"))
	require.NoError(t, err)

	hostCatalogAttributes := &hostcatalogs.HostCatalog_Attributes{
		Attributes: wrapMap(t, map[string]interface{}{
			cred.ConstProject: project,
			cred.ConstZone:    zone,
		}),
	}

	secrets, err := structpb.NewStruct(map[string]interface{}{})
	require.NoError(t, err)

	cases := []struct {
		name        string
		req         *pb.OnCreateCatalogRequest
		expected    *pb.HostCatalogPersisted
		expectedErr string
	}{
		{
			name:        "nil catalog",
			req:         &pb.OnCreateCatalogRequest{},
			expectedErr: "catalog is nil",
		},
		{
			name: "nil attributes",
			req: &pb.OnCreateCatalogRequest{
				Catalog: &hostcatalogs.HostCatalog{},
			},
			expectedErr: "attributes are required",
		},
		{
			name: "error reading attributes",
			req: &pb.OnCreateCatalogRequest{
				Catalog: &hostcatalogs.HostCatalog{
					Attrs: &hostcatalogs.HostCatalog_Attributes{
						Attributes: new(structpb.Struct),
					},
				},
			},
			expectedErr: "attributes.project: missing required value \"project\"",
		},
		{
			name: "do not persist secrets, use gcloud ADC",
			req: &pb.OnCreateCatalogRequest{
				Catalog: &hostcatalogs.HostCatalog{
					Attrs: hostCatalogAttributes,
				},
			},
			expected: &plugin.HostCatalogPersisted{
				Secrets: secrets,
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require := require.New(t)

			actual, err := p.OnCreateCatalog(ctx, tc.req)
			if tc.expectedErr != "" {
				require.Contains(err.Error(), tc.expectedErr)
				return
			}

			require.NoError(err)
			require.Equal(actual.GetPersisted().Secrets, tc.expected.GetSecrets())
		})
	}
}

func TestUpdateCatalog(t *testing.T) {
	ctx := context.Background()
	p := &GooglePlugin{}

	wd, err := os.Getwd()
	require.NoError(t, err)
	require.NotEmpty(t, wd)
	project, err := parseutil.ParsePath("file://" + filepath.Join(wd, "secrets", "project"))
	require.NoError(t, err)
	zone, err := parseutil.ParsePath("file://" + filepath.Join(wd, "secrets", "zone"))
	require.NoError(t, err)

	hostCatalogAttributes := &hostcatalogs.HostCatalog_Attributes{
		Attributes: wrapMap(t, map[string]interface{}{
			cred.ConstProject: project,
			cred.ConstZone:    zone,
		}),
	}

	secrets, err := structpb.NewStruct(map[string]interface{}{})
	require.NoError(t, err)

	cases := []struct {
		name        string
		req         *pb.OnCreateCatalogRequest
		expected    *pb.HostCatalogPersisted
		expectedErr string
	}{
		{
			name:        "nil catalog",
			req:         &pb.OnCreateCatalogRequest{},
			expectedErr: "catalog is nil",
		},
		{
			name: "nil attributes",
			req: &pb.OnCreateCatalogRequest{
				Catalog: &hostcatalogs.HostCatalog{},
			},
			expectedErr: "attributes are required",
		},
		{
			name: "error reading attributes",
			req: &pb.OnCreateCatalogRequest{
				Catalog: &hostcatalogs.HostCatalog{
					Attrs: &hostcatalogs.HostCatalog_Attributes{
						Attributes: new(structpb.Struct),
					},
				},
			},
			expectedErr: "attributes.project: missing required value \"project\"",
		},
		{
			name: "do not persist secrets, use gcloud ADC",
			req: &pb.OnCreateCatalogRequest{
				Catalog: &hostcatalogs.HostCatalog{
					Attrs: hostCatalogAttributes,
				},
			},
			expected: &pb.HostCatalogPersisted{
				Secrets: secrets,
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			require := require.New(t)

			actual, err := p.OnCreateCatalog(ctx, tc.req)
			if tc.expectedErr != "" {
				require.Contains(err.Error(), tc.expectedErr)
				return
			}

			require.NoError(err)
			require.Equal(actual.GetPersisted().Secrets, tc.expected.GetSecrets())
		})
	}
}
