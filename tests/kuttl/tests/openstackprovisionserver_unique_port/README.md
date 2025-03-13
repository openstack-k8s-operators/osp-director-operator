Q: WHAT IS TESTED HERE?

A:

- Create 1 OpenStackProvisionServer
- Create another OpenStackProvisionServer with a different port
- Create a third OpenStackProvisionServer with a duplicate port, which should fail
- Create 1 OpenStackNet
- Create 1 OpenStackBaremetalSet with a count of 1, and make sure it succeeds and has a unique port
- Create another OpenStackBaremetalSet with a count 1, and make sure it succeeds and has a unique port

NOTE: This test assumes you have two (and only two!) extra OCP workers allocated to your cluster as
      BaremetalHosts, that are not in-use as actual cluster nodes
