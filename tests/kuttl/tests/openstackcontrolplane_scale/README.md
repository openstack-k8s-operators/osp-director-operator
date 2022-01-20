Q: WHAT IS TESTED HERE?

A: 

- Create 6 OpenStackNets and an initially-empty OpenStackControlPlane that only spawns an OpenStackClient
- Verify that predictable IPs have been assigned to the OpenStackClient (in the OpenStackNet and the
  associated OpenStackNetConfig), and that the OpenStackNets' underlying NodeNetworkConfigurationPolicies have
  configured successfully
- Scale the OpenStackControlPlane up to 3 controllers
- Check for IP reservations in the OpenStackNet, associated OpenStackNetConfig and OpenStackVMSet for the 3 
  VirtualMachines that should be created
- Check that the OpenStackClient can SSH to all 3 controllers
- Scale the OpenStackControlPlane down to 1 controller
- Check that the IP reservations are removed from the OpenStackVMSet and OpenStackNetConfig, but remain in the
  OpenStackNet
- Scale the OpenStackControlPlane back to 3 controllers
- Check that the IP reservations in the OpenStackNetConfig and OpenStackVMSet reuse the original IPs from
  the original scale-up
- Delete the OpenStackControlPlane
- Check that all sub-resources (OpenStackVMSets, VirtualMachines, OpenStackClient, etc) are removed
