# This example fetches the TLS certificate chain
# from `example.com` using an HTTP Proxy.
# The Proxy is discovered via environment variables:
# see https://pkg.go.dev/net/http#ProxyFromEnvironment for details.

provider "tls" {
  proxy {
    from_env = true
  }
}

data "tls_certificate" "test" {
  url = "https://example.com"
}
