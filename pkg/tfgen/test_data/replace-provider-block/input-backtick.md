## Provider Parameter Priority

There are multiple ways to specify the provider's parameters.  If overlapping values are configured for the provider, then this is the resolution order:

1. Statically configured in the `provider` block
2. Environment variable (where applicable)
3. Taken from the JSON config file
