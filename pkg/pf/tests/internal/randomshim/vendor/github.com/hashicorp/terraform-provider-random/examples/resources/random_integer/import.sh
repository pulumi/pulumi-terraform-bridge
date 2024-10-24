# Random integers can be imported using the result, min, and max, with an
# optional seed. This can be used to replace a config value with a value
# interpolated from the random provider without experiencing diffs.

# Example (values are separated by a ,):
terraform import random_integer.priority 15390,1,50000