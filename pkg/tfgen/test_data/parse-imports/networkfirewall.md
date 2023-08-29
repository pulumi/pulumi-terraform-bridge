```terraform
import {
  to = aws_networkfirewall_resource_policy.example
  id = "aws_networkfirewall_rule_group.example arn:aws:network-firewall:us-west-1:123456789012:stateful-rulegroup/example"
}
```

Using `pulumi import`, import Network Firewall Resource Policies using the `resource_arn`. For example:

```console
% pulumi import aws_networkfirewall_resource_policy.example aws_networkfirewall_rule_group.example arn:aws:network-firewall:us-west-1:123456789012:stateful-rulegroup/example
```
