Q: WHAT IS TESTED HERE?

A: 

- Create 2 OpenStackNets 
- Create 1 initially-empty OpenStackBaremetalServer and 1 Secret (for user password)
- Scale-up the OpenStackBaremetalSet to 2 BaremetalHosts
- Scale-down the OpenStackBaremetalSet to 1 BaremetalHost
- Scale-up the OpenstackBaremetalSet to 2 BaremetalHosts, again
- Delete the OpenStackBaremetalSet

NOTE: This test assumes you have two (and only two!) extra OCP workers allocated to your cluster as 
      BaremetalHosts, that are not in-use as actual cluster nodes
