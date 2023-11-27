package logging

const (
	// Log verbosity is controlled by the TF_LOG environment variable (set to TRACE, DEBUG, INFO, WARN,
	// ERROR or OFF). By default, INFO-level logs are emitted.
	//
	// See also:
	//
	// - https://developer.hashicorp.com/terraform/plugin/log/writing
	// - https://www.pulumi.com/docs/support/troubleshooting
	tfLogEnvVar = "TF_LOG"
)
