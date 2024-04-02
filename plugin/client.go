package plugin

import (
	"errors"

	computepb "cloud.google.com/go/compute/apiv1/computepb"
	pb "github.com/hashicorp/boundary/sdk/pbs/plugin"
)

const (
	NumberMaxResults = uint32(100)
)

func instanceToHost(instance *computepb.Instance) (*pb.ListHostsResponseHost, error) {
	if instance.GetSelfLink() == "" {
		return nil, errors.New("response integrity error: missing instance self-link")
	}

	result := new(pb.ListHostsResponseHost)

	result.ExternalId = instance.GetSelfLink()
	result.ExternalName = instance.GetName()

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
