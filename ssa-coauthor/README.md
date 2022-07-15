1. there is a mandatory unique field

2. add mandatory unique field (user)

3. add optional field with default value (that the webhook has to make unique) (system)


Options:

1. Optional+Default+Webhook on DockerCluster & DockerClusterTemplate
   => We can't random default on DockerClusterTemplate as we get other UUIDs every time we apply the DockerClusterTemplate,
   because in the local YAML uuid is not set and the default webhook generates another UUID.
2. Optional+Default+Webhook on DockerCluster
   => We see an infinite append operation on subnets in DockerCluster. Reason is probably because in managedFields we only see ownership for the "dummy" value. The dummy value only exists after OpenAPI schema defaulting and before our defaulting webhook. tl;dr we're infinite appending orphaned subnets.
3. mandatory DockerClusterTemplate, Optional+Default+Webhook DockerCluster
   => Works for ClusterClass (as the uuid for the DockerCluster is always set)
   => Similar problems for the non-ClusterClass case as in 2.
   => NEW: with uuid preservation => seems to work, but breaks managed fields?
4. mandatory DockerClusterTemplate, mandatory DockerCluster
   => Works but breaking


AWS Use Cases

[x] If subnets are empty => CAPA creates default subnets and patches AWSCluster

If SecondaryCIDR block is set => CAPA adds a corresponding subnet, if there is no subnet with that CIDR yet

CAPA queries AWS for the subnet and syncs fields from AWS to subnet spec

If subnet.ID == "" => create subnet in AWS and sync fields from new subnet from AWS to subnet spec


Context:
As of today CAPA controller adds fields to existing subnets or entire subnets to the subnets list. This behavior
is not compatible to re-apply independent of if the AWSCluster is applied by `kubectl` or the topology controller.
This is an existing problem, not an problem introduced by ClusterClass. 

Proposal 1:
CAPA webhooks should when updating an object (i.e. re-apply) should carry over the fields from the existing object
in order to avoid losing values set by the controller. This approach does not require to add an additional field nor 
to transform the list into a list map. The downside is that the carry-over behavior could be complex given that the 
validation webhook doesn't implement logic limiting the type of changes that can happen. So the number of possible 
scenarios is very high.

We can reduce the number of scenarios by taking care only of update operations of the topology controller. But this 
does not solve the problem of re-apply and this requires extensive tests for CAPA in order to discovery possible 
edge cases.

Please note that this solution implies the carry-over logic in the webhook must be kept in sync with the reconcile 
subnet logic. Also this solution is specific for CAPA. The specific implementation is not portable, the general approach
probably is.

Proposal 2:


