package plugin

import (
	"context"
	"errors"
	"path"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	pb "github.com/hashicorp/boundary/sdk/pbs/plugin"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	NumberMaxResults = uint32(100)
)

func getInstances(ctx context.Context, setId string, request *computepb.ListInstancesRequest) ([]*pb.ListHostsResponseHost, error) {
	hosts := []*pb.ListHostsResponseHost{}
	c, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "error creating NewInstancesRESTClient for host set id %q: %s", setId, err)
	}

	it := c.List(ctx, request)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "error listing instances for host set id %q: %s", setId, err)
		}
		host, err := instanceToHost(resp, setId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "error processing host results for host set id %q: %s", setId, err)
		}
		hosts = append(hosts, host)
	}
	return hosts, nil
}

func getInstancesForInstanceGroup(ctx context.Context, setId string, setAttr *SetAttributes, catalogAttr *CatalogAttributes) ([]*pb.ListHostsResponseHost, error) {
	instanceGroupName := setAttr.InstanceGroup
	hosts := []*pb.ListHostsResponseHost{}
	groupClient, err := compute.NewInstanceGroupsRESTClient(ctx)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "error creating NewInstanceGroupsRESTClient for host set id %q: %s", setId, err)
	}

	request := &computepb.ListInstancesInstanceGroupsRequest{
		InstanceGroup: instanceGroupName,
		Project:       catalogAttr.Project,
		Zone:          catalogAttr.Zone,
	}
	instances := groupClient.ListInstances(ctx, request)

	for {
		resp, err := instances.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "error listing instances for instance group %s in host set id %q: %s", instanceGroupName, setId, err)
		}

		instanceClient, err := compute.NewInstancesRESTClient(ctx)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "error creating NewInstancesRESTClient for host set id %q: %s", setId, err)
		}

		instance, err := instanceClient.Get(ctx, &computepb.GetInstanceRequest{
			Instance: path.Base(resp.GetInstance()),
			Project:  catalogAttr.Project,
			Zone:     catalogAttr.Zone,
		})
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "error getting instance %s for instance group %s in host set id %q: %s", resp.GetInstance(), instanceGroupName, setId, err)
		}

		host, err := instanceToHost(instance,setId)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "error processing host results for instance %s for instance group %s in host set id %q: %s", resp.GetInstance(), instanceGroupName, setId, err)
		}
		hosts = append(hosts, host)
	}
	return hosts, nil
}

func instanceToHost(instance *computepb.Instance,setId string) (*pb.ListHostsResponseHost, error) {
	if instance.GetSelfLink() == "" {
		return nil, errors.New("response integrity error: missing instance self-link")
	}

	result := new(pb.ListHostsResponseHost)

	result.ExternalId = instance.GetSelfLink()
	result.ExternalName = instance.GetName()
	result.SetIds = append(result.SetIds, setId)

	// Now go through all of the interfaces and log the IP address of
	// every interface.
	for _, iface := range instance.GetNetworkInterfaces() {
		// Populate default IP addresses/DNS name similar to how we do
		// for the entire instance.
		result.IpAddresses = appendDistinct(result.IpAddresses, iface.NetworkIP)

		for _, external := range iface.AccessConfigs {
			result.IpAddresses = appendDistinct(result.IpAddresses, external.NatIP)
		}

		// Add the IPv6 addresses.
		result.IpAddresses = appendDistinct(result.IpAddresses, iface.Ipv6Address)
	}

	// Done
	return result, nil
}

// appendDistinct will append the elements to the slice
// if an element is not nil, empty, and does not exist in slice.
func appendDistinct(slice []string, elems ...*string) []string {
	for _, e := range elems {
		if e == nil || *e == "" || stringInSlice(slice, *e) {
			continue
		}
		slice = append(slice, *e)
	}
	return slice
}

func stringInSlice(s []string, x string) bool {
	for _, y := range s {
		if x == y {
			return true
		}
	}
	return false
}
